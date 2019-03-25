/*
Copyright 2019 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"fmt"

	"google.golang.org/grpc"
	mgo "gopkg.in/mgo.v2"

	"github.com/Multy-io/Multy-back/currencies"
	"github.com/Multy-io/Multy-back/eth/client"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
	"github.com/Multy-io/Multy-back/types"
	nsq "github.com/bitly/go-nsq"
	gosocketio "github.com/graarh/golang-socketio"
	"github.com/jekabolt/slf"
)

// ETHConn is a main struct of package, rename to EthController
type ETHConn struct {
	FirebaseNsqProducer *nsq.Producer // a producer for sending data to clients
	// CliTest      pb.NodeCommunicationsClient
	GRPCClient        pb.NodeCommunicationsClient
	NSQClient         *client.ETHEventHandler
	WatchAddress      chan UserAddress
	blockHandler      client.EthBlockHandler
	transactionStatus client.EthTransactionStatusHandler
	// NSQClientTest client.ETHEventHandler
	// WatchAddressTest chan userAddress

	ETHDefaultGasPrice types.TransactionFeeRateEstimation

	WsServer *gosocketio.Server
}

var log = slf.WithContext("eth").WithCaller(slf.CallerShort)

//InitHandlers init nsq mongo and ws connection to node
// return main client , test client , err
func InitHandlers(dbConf *store.Conf, coinTypes []store.CoinType, nsqAddr string) (*ETHConn, error) {
	//declare pacakge struct
	cli := &ETHConn{}

	cli.WatchAddress = make(chan UserAddress)
	// cli.WatchAddressTest = make(chan userAddress)

	config := nsq.NewConfig()
	p, err := nsq.NewProducer(nsqAddr, config)
	if err != nil {
		return cli, fmt.Errorf("nsq producer: %s", err.Error())
	}

	cli.FirebaseNsqProducer = p
	log.Infof("InitHandlers: nsq.NewProducer: √")

	addr := []string{dbConf.Address}

	mongoDBDial := &mgo.DialInfo{
		Addrs:    addr,
		Username: dbConf.Username,
		Password: dbConf.Password,
	}

	db, err := mgo.DialWithInfo(mongoDBDial)
	if err != nil {
		log.Errorf("RunProcess: can't connect to DB: %s", err.Error())
		return cli, fmt.Errorf("mgo.Dial: %s", err.Error())
	}
	log.Infof("InitHandlers: mgo.Dial: √")

	// HACK: this made to acknowledge that queried data has already inserted to db
	db.SetSafe(&mgo.Safe{
		W:        1,
		WTimeout: 100,
		J:        true,
	})

	usersData = db.DB(dbConf.DBUsers).C(store.TableUsers) // all db tables
	exRate = db.DB(dbConf.DBStockExchangeRate).C("TableStockExchangeRate")

	// main
	txsData = db.DB(dbConf.DBTx).C(dbConf.TableTxsDataETHMain)

	// test
	txsDataTest = db.DB(dbConf.DBTx).C(dbConf.TableTxsDataETHTest)

	//restore state
	restoreState = db.DB(dbConf.DBRestoreState).C(dbConf.TableState)

	// setup main net
	coinTypeMain, err := store.FetchCoinType(coinTypes, currencies.Ether, currencies.ETHMain)
	if err != nil {
		return cli, fmt.Errorf("fetchCoinType: %s", err.Error())
	}
	grpcClient, err := initGrpcClient(coinTypeMain.GRPCUrl)
	if err != nil {
		return cli, fmt.Errorf("initGrpcClient: %s", err.Error())
	}

	cli.GRPCClient = grpcClient

	cli.setGRPCHandlers(currencies.ETHMain, coinTypeMain.AccuracyRange)
	log.Infof("InitHandlers: initGrpcClient: Main: √")

	cli.blockHandler = client.EthBlockHandler{}
	cli.transactionStatus = client.EthTransactionStatusHandler{}

	cli.NSQClient, err = client.NewEventHandler(nsqAddr, &cli.blockHandler, &cli.transactionStatus)

	if err != nil {
		log.Errorf("init NSQclient error: %s", err.Error())
		return cli, err
	}

	return cli, nil
}

func initGrpcClient(url string) (pb.NodeCommunicationsClient, error) {
	conn, err := grpc.Dial(url, grpc.WithInsecure())
	if err != nil {
		log.Errorf("initGrpcClient: grpc.Dial: %s", err.Error())
		return nil, err
	}

	// Create a new  client
	client := pb.NewNodeCommunicationsClient(conn)
	return client, nil
}

// EthTransaction stuct for ws notifications
type Transaction struct {
	TransactionType int    `json:"transactionType"`
	Amount          string `json:"amount"`
	TxID            string `json:"txid"`
	Address         string `json:"address"`
}

// BtcTransactionWithUserID sub-stuct for ws notifications
type TransactionWithUserID struct {
	NotificationMsg *Transaction
	UserID          string
}

type UserAddress struct {
	Address      string
	UserID       string
	WalletIndex  int32
	AddressIndex int32
}
