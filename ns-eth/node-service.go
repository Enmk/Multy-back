/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"fmt"
	"net"
	"sync"
	"time"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jekabolt/slf"
	_ "github.com/jekabolt/slflog"
	"google.golang.org/grpc"
)

var log = slf.WithContext("NodeService").WithCaller(slf.CallerShort)

// NodeService is a main struct of service, handles all events and all logics
type NodeService struct {
	Config     *Configuration
	Instance   *NodeClient
	GRPCserver *Server
	Clients    *sync.Map // address to userid
	Storage    *storage.Storage
	EventManager *server.EventManager
}

// Init initializes Multy instance
func (service *NodeService) Init(conf *Configuration) (*NodeService, error) {
	resyncUrl := fetchResyncUrl(conf.NetworkID)
	conf.ResyncUrl = resyncUrl
	service = &NodeService{
		Config: conf,
	}

	var usersData sync.Map

	usersData.Store("address", "address")
	// store.AddressExtended{
	// 	UserID:       "kek",
	// 	WalletIndex:  1,
	// 	AddressIndex: 2,
	// }
	// )

	// initail initialization of clients data
	service.Clients = &usersData

	log.Infof("Users data initialization done")

	// init gRPC server
	lis, err := net.Listen("tcp", conf.GrpcPort)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err.Error())
	}

	// Creates a new gRPC server
	ethCli := NewClient(&conf.EthConf, service.Clients) //, service.CliMultisig)
	if err != nil {
		return nil, fmt.Errorf("eth.NewClient initialization: %s", err.Error())
	}
	log.Infof("ETH client initialization done")

	service.Instance = ethCli

	// Dial to abi client to reach smart contracts methods
	ABIconn, err := ethclient.Dial(conf.AbiClientUrl)
	if err != nil {
		log.Fatalf("Failed to connect to infura %v", err)
	}

	s := grpc.NewServer()
	srv := Server{
		UsersData:       service.Clients,
		EthCli:          service.Instance,
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
				close(service.Instance.RPCStream)
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
