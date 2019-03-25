package nseth

import (
	"context"
	"net"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/common"
	"github.com/Multy-io/Multy-back/common/eth"
)

type ServerRequestHandler interface {
	ServerGetTransaction(eth.TransactionHash) (*eth.Transaction, error)
	ServerGetServiceInfo() common.ServiceInfo

	ServerSetUserAddresses([]eth.Address) error
	ServerResyncAddress(eth.Address) error
}

// Server implements streamer interface and is a gRPC server
type Server struct {
	EthCli          *NodeClient
	gRPCserver      *grpc.Server
	listener        net.Listener
	ReloadChan      chan struct{}

	RequestHandler  ServerRequestHandler
}

func NewServer(grpcPort string, nodeClient *NodeClient, requestHandler  ServerRequestHandler) (server *Server, err error) {
	// init gRPC server
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to listen on %s", grpcPort)
	}

	gRPCserver := grpc.NewServer()
	result := &Server{
		EthCli:         nodeClient,
		gRPCserver:     gRPCserver,
		listener:       lis,
		ReloadChan:     make(chan struct{}),
		RequestHandler: requestHandler,
	}
	pb.RegisterNodeCommunicationsServer(gRPCserver, result)

	return result, nil
}

func (server *Server) Serve() {
	server.gRPCserver.Serve(server.listener)
}

func (server *Server) Stop() error {
	err := server.listener.Close()
	if err != nil {
		return errors.Wrapf(err, "Faield to close listener")
	}

	server.gRPCserver.Stop()
	return nil
}

func (s *Server) GetServiceVersion(c context.Context, in *pb.Empty) (*pb.ServiceVersion, error) {
	info := s.RequestHandler.ServerGetServiceInfo()

	return &pb.ServiceVersion{
		Branch:    info.Branch,
		Commit:    info.Commit,
		Buildtime: info.Buildtime,
		Lasttag:   info.Lasttag,
	}, nil
}

func (s *Server) GetFeeRateEstimation(c context.Context, in *pb.Address) (*pb.FeeRateEstimation, error) {
	gasPriceEstimate := s.EthCli.EstimateTransactionGasPrice()
	feeRateEstimation := &pb.FeeRateEstimation{
		VerySlow:   gasPriceEstimate.VerySlow,
		Slow:       gasPriceEstimate.Slow,
		Medium:     gasPriceEstimate.Medium,
		Fast:       gasPriceEstimate.Fast,
		VeryFast:   gasPriceEstimate.VeryFast,
		IsContract: false,
	}
	// TODO: check if address is cmartcontract then change isContract to true
	code, err := s.EthCli.GetCode(in.Address)
	if err != nil {
		log.Errorf("restClient.ETH.CliMain.EventGetCode falied with error: %v", err.Error())
		return feeRateEstimation, err
	}
	if len(code) > 10 {
		feeRateEstimation.IsContract = true
	}

	return feeRateEstimation, nil
}

func (s *Server) GetAddressInfo(c context.Context, in *pb.Address) (*pb.AddressInfo, error) {
	nonce, err := s.EthCli.GetAddressNonce(in.GetAddress())
	if err != nil {
		return &pb.AddressInfo{}, err
	}

	balance, err := s.EthCli.GetAddressBalance(in.GetAddress())
	if err != nil {
		return &pb.AddressInfo{}, err
	}

	pendingBalance, err := s.EthCli.GetAddressPendingBalance(in.GetAddress())
	if err != nil {
		return &pb.AddressInfo{}, err
	}
	return &pb.AddressInfo{
		Nonce:          nonce.Uint64(),
		Balance:        balance.String(),
		PendingBalance: pendingBalance.String(),
	}, nil
}

func (s *Server) ResyncAddress(c context.Context, address *pb.Address) (*pb.ReplyInfo, error) {
	err := s.RequestHandler.ServerResyncAddress(eth.HexToAddress(address.Address))
	if err != nil {
		return nil, err
	}

	return &pb.ReplyInfo{
		Message: "ok",
	}, nil
}

func (s *Server) CheckRejectTxs(c context.Context, txs *pb.TxsToCheck) (*pb.RejectedTxs, error) {
	reTxs := &pb.RejectedTxs{}
	for _, tx := range txs.Hash {
		rtx, _ := s.EthCli.Rpc.EthGetTransactionByHash(tx)
		if len(rtx.Hash) == 0 {
			reTxs.RejectedTxs = append(reTxs.RejectedTxs, tx)
		}
	}
	return reTxs, nil
}

func (s *Server) GetTransaction(c context.Context, transactionHash *pb.TransactionHash) (*pb.ETHTransaction, error) {
	transaction, err := s.RequestHandler.ServerGetTransaction(eth.HexToHash(transactionHash.Hash))
	if err != nil {
		return nil, err
	}

	return pb.TransactionToProtobuf(*transaction)
}

func (s *Server) SyncState(c context.Context, in *pb.BlockHeight) (*pb.ReplyInfo, error) {
	currentH, err := s.EthCli.GetBlockHeight()
	if err != nil {
		log.Errorf("SyncState:s.EthCli.RpcClient.GetBlockCount: %v ", err.Error())
	}

	log.Debugf("currentH %v lastH %v dif %v", currentH, in.GetHeight(), int64(currentH)-in.GetHeight())

	for lastH := int(in.GetHeight()); lastH < currentH; lastH++ {
		b, err := s.EthCli.Rpc.EthGetBlockByNumber(lastH, true)
		if err != nil {
			log.Errorf("SyncState:s.EthCli.RpcClient.GetBlockHash: %v", err.Error())
		}
		go s.EthCli.ResyncBlock(b)
	}

	return &pb.ReplyInfo{
		Message: "ok",
	}, nil
}

// TODO: Refactor this method and
func (s *Server) EventInitialAdd(c context.Context, ud *pb.UsersData) (*pb.ReplyInfo, error) {

	addresses := ud.GetAddresses()
	if addresses == nil {
		return nil, errors.Errorf("EventInitialAdd addresses is nil")
	}

	log.Debugf("EventInitialAdd total addresses: %d", len(addresses))
	ethAddresses := make([]eth.Address, 0, len(addresses))
	for _, addr := range addresses {
		ethAddress := eth.HexToAddress(addr.Address)
		ethAddresses = append(ethAddresses, ethAddress)
	}

	err := s.RequestHandler.ServerSetUserAddresses(ethAddresses)
	if err != nil {
		return nil, err
	}

	return &pb.ReplyInfo{
		Message: "ok",
	}, nil
}

func (self *Server) SendRawTransaction(c context.Context, tx *pb.RawTransaction) (*pb.ReplyInfo, error) {
	hash, err := self.EthCli.SendRawTransaction(tx.GetRawTx())
	if err != nil {
		return &pb.ReplyInfo{
			Message: "error: wrong raw tx",
		}, fmt.Errorf("error: wrong raw tx %v", err)
	}
	return &pb.ReplyInfo{
		Message: hash,
	}, nil
}

// // TODO: Pasha Change method to return len Message or rename method to 'isContranct' and return boot value
// func (s *Server) EventGetCode(c context.Context, in *pb.AddressToResync) (*pb.ReplyInfo, error) {
// 	code, err := s.EthCli.GetCode(in.Address)
// 	if err != nil {
// 		return &pb.ReplyInfo{}, err
// 	}
// 	return &pb.ReplyInfo{
// 		Message: code,
// 	}, nil
// }
// func (s *Server) GetERC20Info(c context.Context, in *pb.ERC20Address) (*pb.ERC20Info, error) {
// 	addressInfo := &pb.ERC20Info{}

// 	// erc token tx hisotry
// 	url := s.EtherscanAPIURL + "/api?module=account&action=tokentx&address=" + in.GetAddress() + "&startblock=0&endblock=999999999&sort=asc&apikey=" + s.EtherscanAPIKey
// 	request := gorequest.New()
// 	resp, _, errs := request.Get(url).Retry(10, 3*time.Second, http.StatusForbidden, http.StatusBadRequest, http.StatusInternalServerError).End()
// 	if len(errs) > 0 {
// 		return nil, fmt.Errorf("GetERC20Info: request.Get: err: %v", errs[0].Error())
// 	}

// 	respBody, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, fmt.Errorf("GetERC20Info: ioutil.ReadAll: err: %v", err)
// 	}

// 	tokenresp := store.EtherscanResp{}
// 	if err := json.Unmarshal(respBody, &tokenresp); err != nil {
// 		return nil, fmt.Errorf("GetERC20Info: ioutil.ReadAll: err: %v", err)
// 	}
// 	for _, tx := range tokenresp.Result {
// 		addressInfo.History = append(addressInfo.History, &tx)
// 	}

// 	// erc token balances
// 	tokens := map[string]string{}
// 	for _, token := range tokenresp.Result {
// 		tokens[token.ContractAddress] = ""
// 	}
// 	for contract := range tokens {
// 		token, err := NewToken(common.HexToAddress(contract), s.ABIcli)
// 		if err != nil {
// 			log.Errorf("GetERC20ContractInfo - %v", err)
// 			return nil, err
// 		}
// 		balance, err := token.BalanceOf(&bind.CallOpts{}, common.HexToAddress(in.GetAddress()))
// 		if err != nil {
// 			log.Errorf("GetERC20Info:token.BalanceOf %v", err.Error())
// 		}

// 		balanceToSend := "0"
// 		if balance != nil {
// 			balanceToSend = balance.String()
// 		}
// 		addressInfo.Balances = append(addressInfo.Balances, &pb.ERC20Balances{
// 			Address: contract,
// 			Balance: balanceToSend,
// 		})
// 	}

// 	if in.OnlyBalances {
// 		addressInfo.History = nil
// 	}

// 	return addressInfo, nil
// }

// // EventAddNewAddress us used to add new watch address to existing pairs
// func (s *Server) EventAddNewAddress(c context.Context, wa *pb.WatchAddress) (*pb.ReplyInfo, error) {
// 	newMap := *s.UsersData
// 	// if newMap == nil {
// 	// 	newMap = sync.Map{}
// 	// }
// 	_, ok := newMap.Load(wa.Address)
// 	if ok {
// 		return &pb.ReplyInfo{
// 			Message: "err: Address already binded",
// 		}, nil
// 	}
// 	newMap.Store(strings.ToLower(wa.Address), store.AddressExtended{
// 		UserID:       wa.UserID,
// 		WalletIndex:  int(wa.WalletIndex),
// 		AddressIndex: int(wa.AddressIndex),
// 	})

// 	*s.UsersData = newMap

// 	log.Debugf("EventAddNewAddress - %v", newMap)

// 	return &pb.ReplyInfo{
// 		Message: "ok",
// 	}, nil

// }

// func (s *Server) EventGetBlockHeight(c context.Context, in *pb.Empty) (*pb.BlockHeight, error) {
// 	h, err := s.EthCli.GetBlockHeight()
// 	if err != nil {
// 		return &pb.BlockHeight{}, err
// 	}
// 	return &pb.BlockHeight{
// 		Height: int64(h),
// 	}, nil
// }

// func (s *Server) EventGetAdressNonce(c context.Context, in *pb.AddressToResync) (*pb.Nonce, error) {
// 	n, err := s.EthCli.GetAddressNonce(in.GetAddress())
// 	if err != nil {
// 		return &pb.Nonce{}, err
// 	}
// 	return &pb.Nonce{
// 		Nonce: int64(n),
// 	}, nil
// }

// func (s *Server) EventGetAdressBalance(c context.Context, in *pb.AddressToResync) (*pb.Balance, error) {
// 	b, err := s.EthCli.GetAddressBalance(in.GetAddress())
// 	if err != nil {
// 		return &pb.Balance{}, err
// 	}
// 	p, err := s.EthCli.GetAddressPendingBalance(in.GetAddress())
// 	if err != nil {
// 		return &pb.Balance{}, err
// 	}
// 	return &pb.Balance{
// 		Balance:        b.String(),
// 		PendingBalance: p.String(),
// 	}, nil
// }

// func (s *Server) EventSendRawTx(c context.Context, tx *pb.RawTx) (*pb.ReplyInfo, error) {
// 	hash, err := s.EthCli.SendRawTransaction(tx.GetTransaction())
// 	if err != nil {
// 		return &pb.ReplyInfo{
// 			Message: "err: wrong raw tx",
// 		}, fmt.Errorf("err: wrong raw tx %s", err.Error())
// 	}

// 	return &pb.ReplyInfo{
// 		Message: hash,
// 	}, nil

// }

// func (s *Server) NewTx(_ *pb.Empty, stream pb.NodeCommunications_NewTxServer) error {
// 	for tx := range s.EthCli.TransactionsStream {
// 		log.Infof("NewTx history - %v", tx.String())
// 		err := stream.Send(&tx)
// 		if err != nil && err.Error() == ErrGrpcTransport {
// 			log.Warnf("NewTx:stream.Send() %v ", err.Error())
// 			s.ReloadChan <- struct{}{}
// 			return nil
// 		}
// 	}
// 	return nil
// }

// func (s *Server) EventNewBlock(_ *pb.Empty, stream pb.NodeCommunications_EventNewBlockServer) error {
// 	for h := range s.EthCli.BlockStream {
// 		log.Infof("New block height - %v", h.GetHeight())
// 		err := stream.Send(&h)
// 		if err != nil && err.Error() == ErrGrpcTransport {
// 			log.Warnf("EventNewBlock:stream.Send() %v ", err.Error())
// 			s.ReloadChan <- struct{}{}
// 			return nil
// 		}
// 	}
// 	return nil
// }
