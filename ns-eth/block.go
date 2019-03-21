package nseth

import (
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"

	"github.com/onrik/ethrpc"
)

// update mempool every ~5 minutes = 20 block
const blockLengthForReloadTxpool = 20

func (c *Client) BlockTransaction(hash string) {
	block, err := c.Rpc.EthGetBlockByHash(hash, true)
	if err != nil {
		log.Errorf("Get Block Err:%s", err.Error())
		return
	}

	go func(blockNum int64) {
		log.Debugf("new block number = %v", blockNum)
		c.BlockStream <- pb.BlockHeight{
			Height: blockNum,
		}
	}(int64(block.Number))

	txs := []ethrpc.Transaction{}
	if block.Transactions != nil {
		txs = block.Transactions
	} else {
		return
	}

	log.Debugf("New block transaction lenght: %d", len(txs))

	if (c.MempoolReloadBlock + blockLengthForReloadTxpool) < block.Number {
		go c.ReloadTxPool()
		c.MempoolReloadBlock = block.Number
	}

	for _, rawTx := range txs {
		c.parseETHTransaction(rawTx, rawTx.BlockNumber, false)
		c.DeleteTxpoolTransaction(rawTx.Hash)

	}
}

func (c *Client) ResyncBlock(block *ethrpc.Block) {
	log.Warnf("ResyncBlock: %v", block.Number)
	txs := []ethrpc.Transaction{}
	if block.Transactions != nil {
		txs = block.Transactions
	} else {
		log.Errorf("Re-synced block have no transactions on height: %v ", block.Number)
		return
	}

	for _, rawTx := range txs {
		c.parseETHTransaction(rawTx, rawTx.BlockNumber, false)
	}
}
