package eth

import (
	"context"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/common"
	ethcommon "github.com/Multy-io/Multy-back/common/eth"
)

func (self *EthController) GetFeeRate(address ethcommon.Address) (common.TransactionFeeRateEstimation, ethcommon.GasLimit) {
	var gasLimit ethcommon.GasLimit = 21000
	// For testnet we return estimate gas price from mainnet because testnet have many strange transaction
	gasPriceEstimateFromNS, err := self.GRPCClient.GetFeeRateEstimation(context.Background(), &pb.Address{
		Address: address.Hex(),
	})
	if err != nil {
		log.Errorf("wrong ETH eventGasPrice error: %v", err)
		return common.TransactionFeeRateEstimation{
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

	return common.TransactionFeeRateEstimation{
		VerySlow: gasPriceEstimateFromNS.VerySlow,
		Slow:     gasPriceEstimateFromNS.Slow,
		Medium:   gasPriceEstimateFromNS.Medium,
		Fast:     gasPriceEstimateFromNS.Fast,
		VeryFast: gasPriceEstimateFromNS.VeryFast,
	}, gasLimit
}

func (self *EthController) SendRawTransaction(rawTransaction ethcommon.RawTransaction) error {
	log.Infof("raw transaction: %v", rawTransaction)

	return self.NSQClient.EmitRawTransactionEvent(rawTransaction)
}
