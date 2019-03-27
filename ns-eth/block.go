package nseth

import (
	"github.com/onrik/ethrpc"
)

// update mempool every ~5 minutes = 20 block
const blockLengthForReloadTxpool = 20

// HandleNewHeadBlock processes the new top or 'head' block of the chain,
// note that this method is called only when head of the chain updated.
func (c *NodeClient) HandleNewHeadBlock(hash string) {
	block, err := c.Rpc.EthGetBlockByHash(hash, true)
	if err != nil {
		log.Errorf("Failed to fetch block by hash (%s): %+v", hash, err)
		return
	}
	if block == nil {
		log.Errorf("Failed to fetch block by hash (%s): NULL BLOCK", hash)
		return
	}

	blockHeader := rpcBlockToBlockHeader(*block)
	// Run as goroutine to not block if channel is full.
	go func() {
		c.blockStream <- blockHeader
	}()

	txs := []ethrpc.Transaction{}
	if block.Transactions != nil {
		txs = block.Transactions
	} else {
		log.Infof("No transactions in block: %s", block.Hash)
		return
	}

	log.Debugf("New block with %d transactions", len(txs))

	if (c.MempoolReloadBlock + blockLengthForReloadTxpool) < block.Number {
		go c.ReloadTxPool()
		c.MempoolReloadBlock = block.Number
	}

	// TODO: there are many transactions should we start all that in goroutines and use sync.WaitGroup()?
	for _, rawTx := range txs {
		err := c.HandleEthTransaction(rawTx, &blockHeader, false)
		if err != nil {
			log.Errorf("Failed to handle a transaction %s from block %s : %+v",
				rawTx.Hash, hash, err)
		}

		c.DeleteTxpoolTransaction(rawTx.Hash)
	}
}

func (c *NodeClient) ResyncBlock(block *ethrpc.Block) {
	log.Warnf("ResyncBlock: %v", block.Number)
	txs := []ethrpc.Transaction{}
	if block.Transactions != nil {
		txs = block.Transactions
	} else {
		log.Errorf("Re-synced block have no transactions on height: %v ", block.Number)
		return
	}

	blockHeader := rpcBlockToBlockHeader(*block)
	for _, rawTx := range txs {
		c.HandleEthTransaction(rawTx, &blockHeader, false)
	}
}
