/*
Copyright 2017 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/onrik/ethrpc"
	_ "github.com/jekabolt/slflog"
	
	"github.com/Multy-io/Multy-back/types/eth"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
)

type AddressLookup interface {
	IsAddressExists(address eth.Address) bool
}

// TODO: rename to NodeClient
type NodeClient struct {
	Rpc                *ethrpc.EthRPC
	Client             *rpc.Client
	config             *Conf
	TransactionsStream chan eth.Transaction
	BlockStream        chan pb.BlockHeight
	RPCStream          chan interface{}
	Done               <-chan interface{}
	Stop               chan struct{}
	ready              chan struct{} // signalled once when the client is ready
	addressLookup      AddressLookup
	AbiClient          *ethclient.Client
	Mempool            *sync.Map
	MempoolReloadBlock int
}

type Conf struct {
	Address  string
	RpcPort  string
	WsPort   string
	WsOrigin string
}

func NewClient(conf *Conf, addressLookup AddressLookup) *NodeClient {

	c := &NodeClient{
		config:             conf,
		TransactionsStream: make(chan eth.Transaction, 1000),
		BlockStream:        make(chan pb.BlockHeight),
		Done:               make(chan interface{}),
		Stop:               make(chan struct{}),
		ready:              make(chan struct{}, 1), // writing a single event shouldn't block even if nobody listens.
		addressLookup:      addressLookup,
		Mempool:            &sync.Map{},
	}

	go c.RunProcess()
	return c
}

func (c *NodeClient) Shutdown() {
	log.Info("Closing connection to ETH Node.")
	c.Client.Close()
}

func waitForSubCancellation(sub *rpc.ClientSubscription, name string) error {
	for {
		select {
		case err := <-sub.Err():
			log.Warnf("Got a subscription error on %s: %+v", name, err)
			return err
		}
	}
	return nil
}

func (c *NodeClient) RunProcess() error {
	log.Info("Run ETH Process")
	c.Rpc = ethrpc.NewEthRPC("http" + c.config.Address + c.config.RpcPort)
	log.Infof("ETH RPC Connection %s", "http"+c.config.Address+c.config.RpcPort)

	// TODO: why are we even do that? to check connectibility?
	_, err := c.Rpc.EthNewPendingTransactionFilter()
	if err != nil {
		log.Errorf("NewClient:EthNewPendingTransactionFilter: %s", err.Error())
		return err
	}
	height, err := c.GetBlockHeight()
	if err != nil {
		log.Errorf("get block Height err: %v", err)
		height = 1
	}
	c.MempoolReloadBlock = height
	go c.ReloadTxPool()
	client, err := rpc.DialWebsocket(context.TODO(), "ws"+c.config.Address+c.config.WsPort, c.config.WsOrigin)

	if err != nil {
		log.Errorf("Dial err: %s", err.Error())
		return err
	}
	c.Client = client
	log.Infof("ETH RPC Connection %s", "ws"+c.config.Address+c.config.WsPort)

	c.RPCStream = make(chan interface{})

	// Subscribe to node events, for details see https://github.com/ethereum/go-ethereum/wiki/RPC-PUB-SUB
	// TODO: handle errors via context.Err() channel
	sub1, err := c.Client.Subscribe(context.Background(), "eth", c.RPCStream, "newHeads")
	if err != nil {
		log.Errorf("Run: client.Subscribe: newHeads %s", err.Error())
		return err
	}

	sub2, err := c.Client.Subscribe(context.Background(), "eth", c.RPCStream, "newPendingTransactions")
	if err != nil {
		log.Errorf("Run: client.Subscribe: newPendingTransactions %s", err.Error())
		return err
	}

	// done := or(c.fanIn(ch)...)

	// c.Done = done

	go waitForSubCancellation(sub1, "newHeads")
	go waitForSubCancellation(sub2, "newPendingTransactions")
	c.ready <- struct{}{}

	for {
		switch v := (<-c.RPCStream).(type) {
		default:
			log.Errorf("Not found type: %v", v)
		case string:
			go c.AddTransactionToTxpool(v)
		case map[string]interface{}:
			go c.HandleNewHeadBlock(v["hash"].(string))
		case nil:
			// defer func() {
			// 	c.Stop <- struct{}{}
			// }()
			defer client.Close()
			log.Debugf("RPC stream closed")
			return nil
		}
	}

}
