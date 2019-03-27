/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"fmt"
	"net"
	"time"

	"github.com/pkg/errors"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jekabolt/slf"
	"google.golang.org/grpc"
	_ "github.com/jekabolt/slflog"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/common/eth"
	"github.com/Multy-io/Multy-back/ns-eth/storage"
)

var log = slf.WithContext("NodeService").WithCaller(slf.CallerShort)

// NodeService is a main struct of service, handles all events and all logics
type NodeService struct {
	Config       *Configuration
	nodeClient   *NodeClient
	GRPCserver   *Server
	storage      *storage.Storage
	eventManager *EventManager

	lastSeenBlockHeader *eth.BlockHeader
}

type addressLookup struct {
	addressStorage *storage.AddressStorage
	defaultResponse bool
}
func (a *addressLookup) IsKnownAddress(address eth.Address) bool {
	if a.addressStorage != nil {
		return a.addressStorage.IsAddressExists(address)
	}

	return a.defaultResponse
}


// Init initializes Multy instance
func (service *NodeService) Init(conf *Configuration) (*NodeService, error) {
	resyncUrl := fetchResyncUrl(conf.NetworkID)
	conf.ResyncUrl = resyncUrl
	service = &NodeService{
		Config: conf,
	}
	storageInstance, err := storage.NewStorage(conf.DB)
	if err != nil {
		return nil, errors.WithMessagef(err, "Failed to connect to DB")
	}
	service.storage = storageInstance

	// init gRPC server
	lis, err := net.Listen("tcp", conf.GrpcPort)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err.Error())
	}

	// New session to the node
	addressLookup := addressLookup{
		addressStorage:  nil,//service.storage.AddressStorage,
		defaultResponse: true,
	}
	service.nodeClient = NewClient(&conf.EthConf, &addressLookup, service, service)
	if err != nil {
		return nil, fmt.Errorf("eth.NewClient initialization: %s", err.Error())
	}
	log.Infof("ETH client initialization done")

	// Dial to abi client to reach smart contracts methods
	ABIconn, err := ethclient.Dial(conf.AbiClientUrl)
	if err != nil {
		log.Fatalf("Failed to connect to infura %v", err)
	}

	eventManager, err := NewEventManager(conf.NSQURL, service, service)
	if err != nil {
		return nil, err
	}
	service.eventManager = eventManager

	// Creates a new gRPC server
	s := grpc.NewServer()
	srv := Server{
		EthCli:          service.nodeClient,
		Info:            &conf.ServiceInfo,
		NetworkID:       conf.NetworkID,
		ResyncUrl:       resyncUrl,
		EtherscanAPIKey: conf.EtherscanAPIKey,
		EtherscanAPIURL: conf.EtherscanAPIURL,
		ABIcli:          ABIconn,
		GRPCserver:      s,
		Listener:        lis,
		ReloadChan:      make(chan struct{}),
	}

	service.GRPCserver = &srv

	pb.RegisterNodeCommunicationsServer(s, &srv)
	go s.Serve(lis)

	go WatchReload(srv.ReloadChan, service)

	return service, nil
}

// Event Manager Handlers:
func (service *NodeService) HandleNewAddress(address eth.Address) error {
	return service.storage.AddressStorage.AddAddress(address)
}

func (service *NodeService) HandleSendRawTx(rawTx eth.RawTransaction) error {
	_, err := service.nodeClient.SendRawTransaction(string(rawTx))
	// TODO: add a TX hash to a pool of monitored transactions
	return err
}

func (service *NodeService) HandleTransaction(transaction eth.Transaction) {
	err := service.tryHandleTransaction(transaction)
	if err != nil {
		 log.Errorf("Faield to handle a transaction %s : %+v", transaction.Hash.Hex(), err)
	}
}

func (service *NodeService) tryHandleTransaction(transaction eth.Transaction) error {
	// Steps to proceed:
	// decide new status based on current transaction status + block height + current block height
	// save TX to DB
	// emit TX update event (TXID, BlockHash, Status)
	newStatus := decideTransactionStatus(transaction, service.getImmutibleBlockHeight())
	err := service.storage.TransactionStorage.AddTransaction(
		eth.TransactionWithStatus{
			Transaction: transaction,
			Status: newStatus,
		})
	if err != nil {
		return err
	}

	var blockHash eth.BlockHash
	if transaction.BlockInfo != nil {
		blockHash = transaction.BlockInfo.Hash
	}

	return service.eventManager.EmitTransactionStatusEvent(eth.TransactionStatusEvent{
		TransactionHash: transaction.Hash,
		Status:          newStatus,
		BlockHash:       blockHash,
	})
}

func (service *NodeService) HandleBlock(blockHeader eth.BlockHeader) {
	err := service.tryHandleBlock(blockHeader)
	if err != nil {
		log.Errorf("Faield to handle block %s : %+v", blockHeader.Hash.Hex(), err)
	}
}

func (service *NodeService) tryHandleBlock(blockHeader eth.BlockHeader) error {
	err := service.storage.BlockStorage.AddBlock(eth.Block{
		BlockHeader: blockHeader,
	})
	if err != nil {
		return err
	}

	if service.lastSeenBlockHeader == nil || service.lastSeenBlockHeader.Height < blockHeader.Height {
		err := service.storage.BlockStorage.SetLastSeenBlock(blockHeader.Hash)
		if err != nil {
			return err
		}

		service.lastSeenBlockHeader = &blockHeader
	}

	return service.eventManager.EmitNewBlock(blockHeader)
}

func (service *NodeService) getImmutibleBlockHeight() uint64 {
	if service.lastSeenBlockHeader == nil || service.lastSeenBlockHeader.Height < uint64(service.Config.ImmutableBlockDepth) {
		return 0
	}

	return service.lastSeenBlockHeader.Height - uint64(service.Config.ImmutableBlockDepth)
}

func decideTransactionStatus(transaction eth.Transaction, immutibleBlockHeight uint64) eth.TransactionStatus {

	if callInfo := transaction.CallInfo; callInfo != nil {
		if callInfo.Status == eth.SmartContractCallStatusFailed {
			return eth.TransactionStatusErrorSmartContractCallFailed
		}
	}

	if blockInfo := transaction.BlockInfo; blockInfo != nil {
		if blockInfo.Height <= immutibleBlockHeight {
			return eth.TransactionStatusInImmutableBlock
		}
		return eth.TransactionStatusInBlock
	}

	return eth.TransactionStatusInMempool
}

func fetchResyncUrl(networkid int) string {
	switch networkid {
	case 4:
		return "http://api-rinkeby.etherscan.io/api?sort=asc&endblock=99999999&startblock=0&address="
	case 3:
		return "http://api-ropsten.etherscan.io/api?sort=asc&endblock=99999999&startblock=0&address="
	case 1:
		return "http://api.etherscan.io/api?sort=asc&endblock=99999999&startblock=0&address="
	default:
		return "http://api.etherscan.io/api?sort=asc&endblock=99999999&startblock=0&address="
	}
}

func WatchReload(reload chan struct{}, service *NodeService) {
	// func WatchReload(reload chan struct{}, s *grpc.Server, srv *streamer.Server, lis net.Listener, conf *Configuration) {
	for {
		select {
		case _ = <-reload:
			ticker := time.NewTicker(1000 * time.Millisecond)
			err := service.GRPCserver.Listener.Close()
			if err != nil {
				log.Errorf("WatchReload:lis.Close %v", err.Error())
			}
			service.GRPCserver.GRPCserver.Stop()
			log.Debugf("WatchReload:Successfully stopped")
			for _ = range ticker.C {
				close(service.nodeClient.subscriptionsStream)
				_, err := service.Init(service.Config)
				if err != nil {
					log.Errorf("WatchReload:Init %v ", err)
					continue
				}
				log.Debugf("WatchReload:Successfully reloaded")
				return
			}
		}
	}
}