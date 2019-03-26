package nseth

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/ethclient"
	"google.golang.org/grpc"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
)

// Server implements streamer interface and is a gRPC server
type Server struct {
	UsersData       *sync.Map
	EthCli          *NodeClient
	Info            *store.ServiceInfo
	NetworkID       int
	ResyncUrl       string
	EtherscanAPIKey string
	EtherscanAPIURL string
	ABIcli          *ethclient.Client
	GRPCserver      *grpc.Server
	Listener        net.Listener
	ReloadChan      chan struct{}
}

func (s *Server) GetServiceVersion(c context.Context, in *pb.Empty) (*pb.ServiceVersion, error) {
	return &pb.ServiceVersion{
		Branch:    s.Info.Branch,
		Commit:    s.Info.Commit,
		Buildtime: s.Info.Buildtime,
		Lasttag:   "",
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

	// // gasPriceEstimate := s.EthCli.EstimateTransactionGasPrice()
	// return &pb.GasPriceEstimation{
	// 	VerySlow: gasPriceEstimate.VerySlow,
	// 	Slow:     gasPriceEstimate.Slow,
	// 	Medium:   gasPriceEstimate.Medium,
	// 	Fast:     gasPriceEstimate.Fast,
	// 	VeryFast: gasPriceEstimate.VeryFast,
	// }, nil
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

type resyncTx struct {
	Message string `json:"message"`
	Result  []struct {
		Hash string `json:"hash"`
	} `json:"result"`
}

func (s *Server) ResyncAddress(c context.Context, address *pb.Address) (*pb.ReplyInfo, error) {
	log.Debugf("EventResyncAddress")
	addr := address.GetAddress()
	url := s.ResyncUrl + addr + "&action=txlist&module=account"

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &pb.ReplyInfo{
			Message: fmt.Sprintf("EventResyncAddress: http.NewRequest = %s", err.Error()),
		}, nil
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return &pb.ReplyInfo{
			Message: fmt.Sprintf("EventResyncAddress: http.DefaultClient.Do = %s", err.Error()),
		}, nil
	}
	defer res.Body.Close()

	reTx := resyncTx{}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return &pb.ReplyInfo{
			Message: fmt.Sprintf("EventResyncAddress: ioutil.ReadAll = %s", err.Error()),
		}, nil
	}

	if err := json.Unmarshal(body, &reTx); err != nil {
		return &pb.ReplyInfo{
			Message: fmt.Sprintf("EventResyncAddress: json.Unmarshal = %s", err.Error()),
		}, nil
	}

	if !strings.Contains(reTx.Message, "OK") {
		return &pb.ReplyInfo{
			Message: fmt.Sprintf("EventResyncAddress: bad resp form 3-party"),
		}, nil
	}

	log.Debugf("EventResyncAddress %d", len(reTx.Result))

	for _, hash := range reTx.Result {
		s.EthCli.ResyncTransaction(hash.Hash)
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
	// TODO: push that transaction via HandleEthTransaction()
	transaction, err := s.EthCli.Rpc.EthGetTransactionByHash(transactionHash.GetHash())
	if err != nil {
		log.Errorf("GetTransaction: txHash: %s, error: %v", transactionHash.GetHash(), err)
		return nil, err
	}
	return &pb.ETHTransaction{
		Hash:     transaction.Hash,
		From:     transaction.From,
		To:       transaction.To,
		Amount:   transaction.Value.String(),
		Payload:  transaction.Input,
		GasPrice: transaction.GasPrice.Uint64(),
		GasLimit: uint64(transaction.Gas),
		Nonce:    uint64(transaction.Nonce),
		// int64 BlockTime = 9;
		// int64 TxpoolTime = 10;
		BlockHeight: transaction.BlockNumber,
		// bool Resync = 12;
		// string Contract = 18;
		// string Method = 19;
		// string return = 20;
	}, nil

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
	log.Debugf("EventInitialAdd len - %v", len(ud.Map))

	udMap := sync.Map{}
	for addr, ex := range ud.GetMap() {
		udMap.Store(strings.ToLower(addr), ex.GetAddress())
	}

	*s.UsersData = udMap

	return &pb.ReplyInfo{
		Message: "ok",
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
