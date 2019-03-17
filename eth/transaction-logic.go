package eth

import (
	"context"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/types"
	typeseth "github.com/Multy-io/Multy-back/types/eth"
)

func (self *ETHConn) GetFeeRate(address typeseth.Address) (types.TransactionFeeRateEstimation, typeseth.GasLimit) {
	var gasLimit typeseth.GasLimit = 21000
	// For testnet we return estimate gas price from mainnet because testnet have many strange transaction
	gasPriceEstimateFromNS, err := self.GRPCClient.GetFeeRateEstimation(context.Background(), &pb.Address{
		Address: address.Hex(),
	})
	if err != nil {
		log.Errorf("wrong ETH eventGasPrice error: %v", err)
		return types.TransactionFeeRateEstimation{
			VerySlow: self.ETHDefaultGasPrice.VerySlow,
			Slow:     self.ETHDefaultGasPrice.Slow,
			Medium:   self.ETHDefaultGasPrice.Medium,
			Fast:     self.ETHDefaultGasPrice.Fast,
			VeryFast: self.ETHDefaultGasPrice.VeryFast,
		}, gasLimit
	}

	if gasPriceEstimateFromNS.IsContract {
		gasLimit = 40000
	}

	return types.TransactionFeeRateEstimation{
		VerySlow: gasPriceEstimateFromNS.VerySlow,
		Slow:     gasPriceEstimateFromNS.Slow,
		Medium:   gasPriceEstimateFromNS.Medium,
		Fast:     gasPriceEstimateFromNS.Fast,
		VeryFast: gasPriceEstimateFromNS.VeryFast,
	}, gasLimit
}

func (self *ETHConn) SendRawTransaction(rawTransaction typeseth.RawTransaction) error {
	log.Infof("raw transaction: %v", rawTransaction)

	return self.NSQClient.EmitRawTransactionEvent(rawTransaction)
}
