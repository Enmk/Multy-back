package nseth

import (
	"context"
	"fmt"
	"net"

	"github.com/pkg/errors"
	"google.golang.org/grpc"

	"github.com/Multy-io/Multy-back/common"
	"github.com/Multy-io/Multy-back/common/eth"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
)

type ServerRequestHandler interface {
	ServerGetTransaction(eth.TransactionHash) (*eth.Transaction, error)
	ServerGetServiceInfo() common.ServiceInfo

	ServerSetUserAddresses([]eth.Address) error
	ServerResyncAddress(eth.Address) error
}

// Server implements streamer interface and is a gRPC server
type Server struct {
	EthCli     *NodeClient
	gRPCserver *grpc.Server
	listener   net.Listener
	ReloadChan chan struct{}

	RequestHandler ServerRequestHandler
}

func NewServer(grpcPort string, nodeClient *NodeClient, requestHandler ServerRequestHandler) (server *Server, err error) {
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
