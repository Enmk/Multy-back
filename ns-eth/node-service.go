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

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jekabolt/slf"
	"google.golang.org/grpc"
	
	_ "github.com/jekabolt/slflog"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/types/eth"

	"github.com/Multy-io/Multy-back/ns-eth/storage"
	"github.com/Multy-io/Multy-back/ns-eth/server"
)

var log = slf.WithContext("NodeService").WithCaller(slf.CallerShort)

// NodeService is a main struct of service, handles all events and all logics
type NodeService struct {
	Config     *Configuration
	nodeClient   *NodeClient
	GRPCserver *Server
	// clients    *sync.Map // 'set' of addresses (eth.Address => struct{}{})
	storage    *storage.Storage
	eventManager *server.EventManager
}

// Init initializes Multy instance
func (service *NodeService) Init(conf *Configuration) (*NodeService, error) {
	resyncUrl := fetchResyncUrl(conf.NetworkID)
	conf.ResyncUrl = resyncUrl
	service = &NodeService{
		Config: conf,
	}

	log.Infof("Users data initialization done")

	// init gRPC server
	lis, err := net.Listen("tcp", conf.GrpcPort)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err.Error())
	}

	// New session to the node
	ethCli := NewClient(&conf.EthConf, service.storage.AddressStorage)
	if err != nil {
		return nil, fmt.Errorf("eth.NewClient initialization: %s", err.Error())
	}
	log.Infof("ETH client initialization done")

	service.nodeClient = ethCli

	// Dial to abi client to reach smart contracts methods
	ABIconn, err := ethclient.Dial(conf.AbiClientUrl)
	if err != nil {
		log.Fatalf("Failed to connect to infura %v", err)
	}

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

// func (service *NodeService) ProcessTransactionStream() {
// 	// We've faced new transaction:
// 	for tx := range service.TransactionStream {
// 		log.Infof("NewTx history - %v", tx.ID)
// 	}
// }



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
				close(service.nodeClient.RPCStream)
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