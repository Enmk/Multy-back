package eth

import (
	"context"

	"github.com/pkg/errors"

	ethcommon "github.com/Multy-io/Multy-back/common/eth"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
)

func (self *EthController) GetAddressInfo(address ethcommon.Address) (ethcommon.AddressInfo, error) {
	log := log.WithField("address", address.Hex())

	addressInfo, err := self.GRPCClient.GetAddressInfo(context.Background(), &pb.Address{
		Address: address.Hex(),
	})
	if err != nil {
		log.Errorf("Error on ns-GRPC GetAddressInfo error: %v", err)
		return ethcommon.AddressInfo{}, err
	}
	balance, err := ethcommon.NewAmountFromString(addressInfo.GetBalance(), 10)
	if err != nil {
		log.Errorf("Error on convert amount string to eth.Amount value: %v error: %v ", addressInfo.GetBalance(), err)
		return ethcommon.AddressInfo{}, err
	}
	pendingBalance, err := ethcommon.NewAmountFromString(addressInfo.GetPendingBalance(), 10)
	if err != nil {
		log.Errorf("Error on convert amount string to eth.Amount value: %v error: %v ", addressInfo.GetBalance(), err)
		return ethcommon.AddressInfo{}, err
	}

	return ethcommon.AddressInfo{
		TotalBalance:   *balance,
		PendingBalance: *pendingBalance,
		Nonce:          ethcommon.TransactionNonce(addressInfo.Nonce),
	}, nil
}

func (self *EthController) ResyncAddress(address ethcommon.Address) error {
	_, err := self.GRPCClient.ResyncAddress(context.Background(), &pb.Address{
		Address: address.Hex(),
	})

	return err
}


func (self *EthController) GetUserTransactions(user store.User, walletId, currencyId, networkId int, token *ethcommon.Address) ([]store.TransactionETH, error) {
	userTxs := []store.TransactionETH{}

	tokenAddress := ""
	if token != nil {
		tokenAddress = token.Hex()
	}

	err := self.userStore.GetAllWalletEthTransactions(user.UserID, currencyId, networkId, walletId, tokenAddress, &userTxs)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get all user wallets from DB.")
	}

	blockHeight, err := self.GetBlockHeigth()
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to get block height from DB.")
	}

	for i := 0; i < len(userTxs); i++ {
		if userTxs[i].BlockTime == 0 {
			userTxs[i].Confirmations = 0
		} else if userTxs[i].BlockTime != 0 {
			userTxs[i].Confirmations = int(blockHeight-userTxs[i].BlockHeight) + 1
		}
	}

	return userTxs, nil
}