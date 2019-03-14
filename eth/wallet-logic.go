package eth

import (
	"context"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	typeseth "github.com/Multy-io/Multy-back/types/eth"
)

func (self *ETHConn) GetAddressInfo(address typeseth.Address) (typeseth.AddressInfo, error) {
	addressInfo, err := self.GRPCClient.GetAddressInfo(context.Background(), &pb.Address{
		Address: string(address),
	})
	if err != nil {
		log.Errorf("Error on ns-GRPC GetAddressInfo address: %v error: %v ", address, err)
		return typeseth.AddressInfo{}, err
	}
	balance, err := typeseth.NewAmountFromString(addressInfo.GetBalance(), 10)
	if err != nil {
		log.Errorf("Error on convert amount string to eth.Amount value: %v error: %v ", addressInfo.GetBalance(), err)
		return typeseth.AddressInfo{}, err
	}
	pendingBalance, err := typeseth.NewAmountFromString(addressInfo.GetPendingBalance(), 10)
	if err != nil {
		log.Errorf("Error on convert amount string to eth.Amount value: %v error: %v ", addressInfo.GetBalance(), err)
		return typeseth.AddressInfo{}, err
	}

	return typeseth.AddressInfo{
		TotalBalance:   *balance,
		PendingBalance: *pendingBalance,
		Nonce:          typeseth.TransactionNonce(addressInfo.Nonce),
	}, nil
}

func (self *ETHConn) ResyncAddress(address typeseth.Address) error {
	_, err := self.GRPCClient.ResyncAddress(context.Background(), &pb.Address{
		Address: string(address),
	})

	return err
}
