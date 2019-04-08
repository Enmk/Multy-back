/*
Copyright 2017 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"context"
	"sync"

	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	_ "github.com/jekabolt/slflog"
	"github.com/onrik/ethrpc"

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

type Reconnector interface {
	RequestReconnect(error)
}


type NodeClient struct {
	Rpc                 *ethrpc.EthRPC
	Client              *rpc.Client
	config              *Conf
	transactionsStream  chan eth.Transaction
	blockStream         chan eth.BlockHeader
	subscriptionsStream chan interface{}
	Done                <-chan interface{}
	Stop                chan struct{}
	AbiClient           *ethclient.Client
	Mempool             *sync.Map
	MempoolReloadBlock  int
	reconnector         Reconnector

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

func NewClient(conf *Conf, addressLookup AddressLookup, txHandler TransactionHandler,
		blockHandler BlockHandler, reconnector Reconnector) (*NodeClient, error) {

	c := &NodeClient{
		config:             conf,
		transactionsStream: make(chan eth.Transaction, 1000),
		blockStream:        make(chan eth.BlockHeader, 10),
		Done:               make(chan interface{}),
		Stop:               make(chan struct{}),
		Mempool:            &sync.Map{},
		addressLookup:      addressLookup,
		transactionHandler: txHandler,
		blockHandler:       blockHandler,
		reconnector:        reconnector,
	}

	err := c.StartProcess()
	if err != nil {
		return nil, err
	}

	return c, nil
}

func (c *NodeClient) Shutdown() {
	log.Info("Closing connection to ETH Node.")
	if c.Client != nil {
		c.Client.Close()
	}
	if c.subscriptionsStream != nil {
		close(c.subscriptionsStream)
	}
}

func (c *NodeClient) waitForSubCancellation(sub *rpc.ClientSubscription, name string) {
	err, _ := <-sub.Err()
	log.Warnf("Got a subscription error on %s: %+v", name, err)

	c.reconnector.RequestReconnect(err)
}

func (c *NodeClient) StartProcess() error {
	log.Info("Run ETH Process")

	go c.processBlocks()
	go c.processTransactions()

	c.Rpc = ethrpc.NewEthRPC("http" + c.config.Address + c.config.RpcPort)
	log.Infof("ETH RPC Connection %s", "http"+c.config.Address+c.config.RpcPort)

	// TODO: !!! why do we do that? to check connectibility?
	// _, err := c.Rpc.EthNewPendingTransactionFilter()
	// if err != nil {
	// 	log.Errorf("NewClient:EthNewPendingTransactionFilter: %s", err.Error())
	// 	return err
	// }

	client, err := rpc.DialWebsocket(context.TODO(), "ws"+c.config.Address+c.config.WsPort, c.config.WsOrigin)
	if err != nil {
		log.Errorf("Dial err: %s", err.Error())
		return err
	}
	c.Client = client
	log.Infof("ETH RPC Connection %s", "ws"+c.config.Address+c.config.WsPort)

	height, err := c.GetBlockHeight()
	if err != nil {
		log.Errorf("get block Height err: %v", err)
		height = 1
	}
	c.MempoolReloadBlock = height
	go c.ReloadTxPool()

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

	go c.waitForSubCancellation(sub1, "newHeads")
	go c.waitForSubCancellation(sub2, "newPendingTransactions")

	go func() {
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
				c.reconnector.RequestReconnect(errors.New("Node RPC stream closed"))
				return
			}
		}
	}()

	return nil
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
