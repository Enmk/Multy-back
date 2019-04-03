/*
Copyright 2019 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"fmt"
	"time"
	"github.com/pkg/errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"github.com/grpc-ecosystem/go-grpc-middleware/retry"
	mgo "gopkg.in/mgo.v2"
	nsq "github.com/bitly/go-nsq"
	gosocketio "github.com/graarh/golang-socketio"
	"github.com/jekabolt/slf"

	// ethcommon "github.com/Multy-io/Multy-back/common/eth"
	"github.com/Multy-io/Multy-back/currencies"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
	"github.com/Multy-io/Multy-back/common"
	"github.com/Multy-io/Multy-back/ns-eth/storage"
)

// EthController is a main struct of package
type EthController struct {
	FirebaseNsqProducer *nsq.Producer // a producer for sending data to clients
	GRPCClient         pb.NodeCommunicationsClient
	eventManager       *EventManager
	WatchAddress       chan UserAddress
	transactionStorage *storage.TransactionStorage
	userStore          store.UserStore
	coinType           store.CoinType

	// blockHandler      BlockHandler
	// transactionStatus TransactionStatusHandler
	// eventManagerTest client.EventManager
	// WatchAddressTest chan userAddress

	ETHDefaultGasPrice common.TransactionFeeRateEstimation

	WsServer *gosocketio.Server
}

var log = slf.WithContext("eth").WithCaller(slf.CallerShort)

//InitHandlers init nsq mongo and ws connection to node
func NewController(dbConf *store.Conf, coinTypes []store.CoinType, nsqAddr string, userStore store.UserStore) (*EthController, error) {
	//declare pacakge struct
	controller := &EthController{
		WatchAddress: make(chan UserAddress),
		userStore: userStore,
	}

	config := nsq.NewConfig()
	p, err := nsq.NewProducer(nsqAddr, config)
	if err != nil {
		return controller, errors.Wrapf(err, "Failed to create nsq producer")
	}

	controller.FirebaseNsqProducer = p
	log.Infof("nsq.NewProducer: √")

	addr := []string{dbConf.Address}

	mongoDBDial := &mgo.DialInfo{
		Addrs:    addr,
		Username: dbConf.Username,
		Password: dbConf.Password,
		Timeout:  dbConf.Timeout,
	}

	db, err := mgo.DialWithInfo(mongoDBDial)
	if err != nil {
		log.Errorf("RunProcess: can't connect to DB: %s", err.Error())
		return controller, errors.Wrapf(err, "Failed to dial to DB")
	}
	log.Infof("mgo.Dial: √")

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

	//restore state
	restoreState = db.DB(dbConf.DBRestoreState).C(dbConf.TableState)

	controller.transactionStorage = storage.NewTransactionStorage(db.DB(dbConf.DBTx).C("BlockchainTransactions"))

	// setup main net
	coinType, err := store.FetchCoinType(coinTypes, currencies.Ether, currencies.ETHMain)
	if err != nil {
		return nil, fmt.Errorf("fetchCoinType: %s", err.Error())
	}
	grpcClient, err := initGrpcClient(coinType.GRPCUrl)
	if err != nil {
		return nil, fmt.Errorf("initGrpcClient: %s", err.Error())
	}

	controller.GRPCClient = grpcClient

	controller.setGRPCHandlers(currencies.ETHMain, coinType.AccuracyRange)
	log.Infof("initGrpcClient: Main: √")

	controller.eventManager, err = NewEventHandler(nsqAddr, controller, controller)

	if err != nil {
		log.Errorf("init NSQclient error: %s", err.Error())
		return nil, err
	}

	return controller, nil
}

func initGrpcClient(url string) (pb.NodeCommunicationsClient, error) {
	backoff := grpc_retry.BackoffExponential(100 * time.Millisecond)
	opts := []grpc_retry.CallOption{
		grpc_retry.WithCodes(codes.Unavailable, codes.DataLoss),
		grpc_retry.WithMax(10),
		grpc_retry.WithBackoff(func(attempt uint) time.Duration {
			duration := backoff(attempt)
			log.Infof("GRPC retry attempt %d : waiting %s", attempt, duration.String())
			return duration
		}),
	}

	conn, err := grpc.Dial(url,
		grpc.WithInsecure(),
		grpc.WithStreamInterceptor(grpc_retry.StreamClientInterceptor(opts...)),
		grpc.WithUnaryInterceptor(grpc_retry.UnaryClientInterceptor(opts...)))

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

// TransactionWithUserID sub-stuct for ws notifications
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
