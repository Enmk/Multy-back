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
	
	"github.com/Multy-io/Multy-back/common/eth"
)

type AddressLookup interface {
	IsKnownAddress(address eth.Address) bool
}

type TransactionHandler interface {
	HandleTransaction(eth.Transaction)
}

type BlockHandler interface {
	HandleBlock(eth.BlockHeader)
}

// TODO: rename to NodeClient
type NodeClient struct {
	Rpc                 *ethrpc.EthRPC
	Client              *rpc.Client
	config              *Conf
	transactionsStream  chan eth.Transaction
	blockStream         chan eth.BlockHeader
	subscriptionsStream chan interface{}
	Done                <-chan interface{}
	Stop                chan struct{}
	ready               chan struct{} // signalled once when the client is ready
	AbiClient           *ethclient.Client
	Mempool             *sync.Map
	MempoolReloadBlock  int

	addressLookup      AddressLookup
	transactionHandler TransactionHandler
	blockHandler       BlockHandler
}

type Conf struct {
	Address  string
	RpcPort  string
	WsPort   string
	WsOrigin string
}

func NewClient(conf *Conf, addressLookup AddressLookup, txHandler TransactionHandler, blockHandler BlockHandler) *NodeClient {

	c := &NodeClient{
		config:             conf,
		transactionsStream: make(chan eth.Transaction, 1000),
		blockStream:        make(chan eth.BlockHeader, 10),
		Done:               make(chan interface{}),
		Stop:               make(chan struct{}),
		ready:              make(chan struct{}, 1), // writing a single event shouldn't block even if nobody listens.
		Mempool:            &sync.Map{},
		addressLookup:      addressLookup,
		transactionHandler: txHandler,
		blockHandler:       blockHandler,
	}

	go c.RunProcess()
	return c
}

func (c *NodeClient) Shutdown() {
	log.Info("Closing connection to ETH Node.")
	c.Client.Close()
}

func waitForSubCancellation(sub *rpc.ClientSubscription, name string) error {
	err, _ := <-sub.Err()
	log.Warnf("Got a subscription error on %s: %+v", name, err)
	return err
}

func (c *NodeClient) RunProcess() error {
	log.Info("Run ETH Process")

	go c.processBlocks()
	go c.processTransactions()

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

	c.subscriptionsStream = make(chan interface{})

	// Subscribe to node events, for details see https://github.com/ethereum/go-ethereum/wiki/RPC-PUB-SUB
	// TODO: handle errors via context.Err() channel
	sub1, err := c.Client.Subscribe(context.Background(), "eth", c.subscriptionsStream, "newHeads")
	if err != nil {
		log.Errorf("Run: client.Subscribe: newHeads %s", err.Error())
		return err
	}

	sub2, err := c.Client.Subscribe(context.Background(), "eth", c.subscriptionsStream, "newPendingTransactions")
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
		switch v := (<-c.subscriptionsStream).(type) {
		default:
			log.Errorf("Not found type: %v", v)
		case string:
			go c.AddTransactionToTxpool(v)
		case map[string]interface{}:
			// Here in `v` we have a block, but with no transaction hashes.
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

func (nodeClient *NodeClient) processTransactions() {
	for {
		tx, ok := <-nodeClient.transactionsStream
		if !ok {
			log.Errorf("Failed to read value from transactionsStream")
			break
		}
		nodeClient.transactionHandler.HandleTransaction(tx)
	}
}

func (nodeClient *NodeClient) processBlocks() {
	for {
		block, ok := <-nodeClient.blockStream
		if !ok {
			log.Errorf("Failed to read value from blockStream")
			break
		}
		nodeClient.blockHandler.HandleBlock(block)
	}
}
