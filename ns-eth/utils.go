/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"math/big"
	"time"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
	"github.com/onrik/ethrpc"
	"gopkg.in/mgo.v2/bson"
)

func newETHtx(hash, from, to string, amount float64, gas, gasprice, nonce int) store.TransactionETH {
	return store.TransactionETH{}
}

func (client *Client) SendRawTransaction(rawTX string) (string, error) {
	hash, err := client.Rpc.EthSendRawTransaction(rawTX)
	if err != nil {
		log.Errorf("SendRawTransaction:rpc.EthSendRawTransaction: %s", err.Error())
		return hash, err
	}
	return hash, err
}

func (client *Client) GetAddressBalance(address string) (big.Int, error) {
	balance, err := client.Rpc.EthGetBalance(address, "latest")
	if err != nil {
		log.Errorf("GetAddressBalance:rpc.EthGetBalance: %s", err.Error())
		return balance, err
	}
	return balance, err
}

func (client *Client) GetTxByHash(hash string) (bool, error) {
	tx, err := client.Rpc.EthGetTransactionByHash(hash)
	if tx == nil {
		return false, err
	} else {
		return true, err
	}
}

func (client *Client) GetAddressPendingBalance(address string) (big.Int, error) {
	balance, err := client.Rpc.EthGetBalance(address, "pending")
	if err != nil {
		log.Errorf("GetAddressPendingBalance:rpc.EthGetBalance: %s", err.Error())
		return balance, err
	}
	log.Debugf("GetAddressPendingBalance %v", balance.String())
	return balance, err
}

func (client *Client) GetAllTxPool() ([]byte, error) {
	return client.Rpc.TxPoolContent()
}

func (client *Client) GetBlockHeight() (int, error) {
	return client.Rpc.EthBlockNumber()
}

func (client *Client) GetCode(address string) (string, error) {
	return client.Rpc.EthGetCode(address, "latest")
}

func (client *Client) GetAddressNonce(address string) (big.Int, error) {
	return client.Rpc.EthGetTransactionCount(address, "latest")
}

func (client *Client) ResyncAddress(txid string) error {
	tx, err := client.Rpc.EthGetTransactionByHash(txid)
	if err != nil {
		return err
	}
	if tx != nil {
		client.parseETHTransaction(*tx, tx.BlockNumber, true)
	}
	return nil
}

func (client *Client) parseETHTransaction(rawTX ethrpc.Transaction, blockHeight int64, isResync bool) {
	var fromUser string
	var toUser string

	if udFrom, ok := client.UsersData.Load(rawTX.From); ok {
		fromUser = udFrom.(string)
	}

	if udTo, ok := client.UsersData.Load(rawTX.To); ok {
		toUser = udTo.(string)
	}

	if toUser == "" && fromUser == "" {
		// not our users tx
		return
	}

	tx := rawToGenerated(rawTX)
	tx.Resync = isResync

	block, err := client.Rpc.EthGetBlockByHash(rawTX.BlockHash, false)
	if err != nil {
		if blockHeight == -1 {
			tx.TxpoolTime = time.Now().Unix()
		} else {
			tx.BlockTime = time.Now().Unix()
		}
		tx.BlockHeight = blockHeight
	} else {
		tx.BlockTime = int64(block.Timestamp)
		tx.BlockHeight = int64(block.Number)
	}

	if blockHeight == -1 {
		tx.TxpoolTime = time.Now().Unix()
	}

	// log.Infof("tx - %v", tx)

	/*
		Fetching tx status and send
	*/
	// from v1 to v1
	// if fromUser == toUser && fromUser != "" {
	// tx.UserID = fromUser.UserID
	// tx.WalletIndex = int32(fromUser.WalletIndex)
	// tx.AddressIndex = int32(fromUser.AddressIndex)

	// **********
	// **** TODO: Send transacton with NSQ
	// **********
	// tx.Status = store.TxStatusAppearedInBlockOutcoming
	// if blockHeight == -1 {
	// 	tx.Status = store.TxStatusAppearedInMempoolOutcoming
	// }
	// // send to multy-back
	// client.TransactionsStream <- tx
	// // }

	// **********
	// **** TODO: Send transacton with NSQ
	// **********

	// from v1 to v2 outgoing
	// if fromUser.UserID != "" {
	// 	tx.UserID = fromUser.UserID
	// 	tx.WalletIndex = int32(fromUser.WalletIndex)
	// 	tx.AddressIndex = int32(fromUser.AddressIndex)
	// 	tx.Status = store.TxStatusAppearedInBlockOutcoming
	// 	if blockHeight == -1 {
	// 		tx.Status = store.TxStatusAppearedInMempoolOutcoming
	// 	}
	// 	log.Warnf("outgoing ----- for uid %v ", fromUser.UserID)
	// 	// send to multy-back
	// 	client.TransactionsStream <- tx
	// }

	// from v1 to v2 incoming
	// if toUser.UserID != "" {
	// 	tx.UserID = toUser.UserID
	// 	tx.WalletIndex = int32(toUser.WalletIndex)
	// 	tx.AddressIndex = int32(toUser.AddressIndex)
	// 	tx.Status = store.TxStatusAppearedInBlockIncoming
	// 	if blockHeight == -1 {
	// 		tx.Status = store.TxStatusAppearedInMempoolIncoming
	// 	}
	// 	log.Warnf("incoming ----- for uid %v ", toUser.UserID)
	// 	// send to multy-back
	// 	client.TransactionsStream <- tx
	// }

}

func rawToGenerated(rawTX ethrpc.Transaction) pb.ETHTransaction {
	return pb.ETHTransaction{
		Hash:     rawTX.Hash,
		From:     rawTX.From,
		To:       rawTX.To,
		Amount:   rawTX.Value.String(),
		GasPrice: uint64(rawTX.GasPrice.Int64()),
		GasLimit: uint64(rawTX.Gas),
		Nonce:    uint64(rawTX.Nonce),
		Payload:  rawTX.Input,
	}
}

func isMempoolUpdate(mempool bool, status int) bson.M {
	if mempool {
		return bson.M{
			"$set": bson.M{
				"status": status,
			},
		}
	}
	return bson.M{
		"$set": bson.M{
			"status":    status,
			"blocktime": time.Now().Unix(),
		},
	}
}
