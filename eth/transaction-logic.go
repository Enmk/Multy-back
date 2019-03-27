package eth

import (
	"context"

	"github.com/Multy-io/Multy-back/common"
	ethcommon "github.com/Multy-io/Multy-back/common/eth"
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
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

func (self *EthController) SendRawTransaction(rawTransaction ethcommon.RawTransaction) (string, error) {
	err := self.NSQClient.EmitRawTransactionEvent(rawTransaction)
	if err != nil {
		log.Errorf("No push raw transaction to NSQ, error: %v", err)
		return "", err
	}
	log.Infof("raw transaction: %v", rawTransaction)
	reply, err := self.GRPCClient.SendRawTransaction(context.Background(), &pb.RawTransaction{
		RawTx: string(rawTransaction),
	})
	if err != nil {
		log.Errorf("send raw transaction from GRPC error: %v", err)
		return reply.GetMessage(), err
	}
	return reply.GetMessage(), nil
}
