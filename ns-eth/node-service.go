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
	"github.com/Multy-io/Multy-back/store"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jekabolt/slf"
	_ "github.com/jekabolt/slflog"
	"google.golang.org/grpc"
)

var log = slf.WithContext("NodeClient").WithCaller(slf.CallerShort)

// NodeClient is a main struct of service
type NodeClient struct {
	Config     *Configuration
	Instance   *Client
	GRPCserver *Server
	Clients    *sync.Map // address to userid
}

// Init initializes Multy instance
func (nc *NodeClient) Init(conf *Configuration) (*NodeClient, error) {
	resyncUrl := fetchResyncUrl(conf.NetworkID)
	conf.ResyncUrl = resyncUrl
	nc = &NodeClient{
		Config: conf,
	}

	var usersData sync.Map

	usersData.Store("address", store.AddressExtended{
		UserID:       "kek",
		WalletIndex:  1,
		AddressIndex: 2,
	})

	// initail initialization of clients data
	nc.Clients = &usersData

	log.Infof("Users data initialization done")

	// init gRPC server
	lis, err := net.Listen("tcp", conf.GrpcPort)
	if err != nil {
		return nil, fmt.Errorf("failed to listen: %v", err.Error())
	}

	// Creates a new gRPC server
	ethCli := NewClient(&conf.EthConf, nc.Clients) //, nc.CliMultisig)
	if err != nil {
		return nil, fmt.Errorf("eth.NewClient initialization: %s", err.Error())
	}
	log.Infof("ETH client initialization done")

	nc.Instance = ethCli

	// Dial to abi client to reach smart contracts methods
	ABIconn, err := ethclient.Dial(conf.AbiClientUrl)
	if err != nil {
		log.Fatalf("Failed to connect to infura %v", err)
	}

	s := grpc.NewServer()
	srv := Server{
		UsersData:       nc.Clients,
		EthCli:          nc.Instance,
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

	nc.GRPCserver = &srv

	pb.RegisterNodeCommunicationsServer(s, &srv)
	go s.Serve(lis)

	go WathReload(srv.ReloadChan, nc)

	return nc, nil
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

func WathReload(reload chan struct{}, cli *NodeClient) {
	// func WathReload(reload chan struct{}, s *grpc.Server, srv *streamer.Server, lis net.Listener, conf *Configuration) {
	for {
		select {
		case _ = <-reload:
			ticker := time.NewTicker(1000 * time.Millisecond)
			err := cli.GRPCserver.Listener.Close()
			if err != nil {
				log.Errorf("WathReload:lis.Close %v", err.Error())
			}
			cli.GRPCserver.GRPCserver.Stop()
			log.Debugf("WathReload:Successfully stopped")
			for _ = range ticker.C {
				close(cli.Instance.RPCStream)
				_, err := cli.Init(cli.Config)
				if err != nil {
					log.Errorf("WathReload:Init %v ", err)
					continue
				}
				log.Debugf("WathReload:Successfully reloaded")
				return
			}
		}
	}
}
