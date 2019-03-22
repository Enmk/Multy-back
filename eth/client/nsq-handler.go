package client

import (
	"github.com/Multy-io/Multy-back/types/eth"
	"github.com/jekabolt/slf"
)

var log = slf.WithContext("eth-NSQ-handler").WithCaller(slf.CallerShort)

type EthBlockHandler struct {
	blockHeight uint64
}

func (self *EthBlockHandler) GetBlockHight() uint64 {
	return self.blockHeight
}

func (self *EthBlockHandler) HandleBlock(block eth.BlockHeader) error {
	self.blockHeight = block.Height
	return nil
}

type EthTransactionStatusHandler struct {
}

func (self *EthTransactionStatusHandler) HandleTxStatus(tx eth.TransactionWithStatus) error {
	log.Infof("new status for tx: %v status: %v", tx.ID, tx.Status)
	return nil
}
