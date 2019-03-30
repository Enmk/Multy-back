package eth

import (
	"context"

	ethcommon "github.com/Multy-io/Multy-back/common/eth"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
)

func (self *EthController) GetAddressInfo(address ethcommon.Address) (ethcommon.AddressInfo, error) {
	addressInfo, err := self.GRPCClient.GetAddressInfo(context.Background(), &pb.Address{
		Address: address.Hex(),
	})
	if err != nil {
		log.Errorf("Error on ns-GRPC GetAddressInfo address: %v error: %v ", address, err)
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
