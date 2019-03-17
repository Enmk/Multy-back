/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"math/big"
	"time"
	"fmt"
	"strings"

	"github.com/pkg/errors"

	"github.com/onrik/ethrpc"
	"gopkg.in/mgo.v2/bson"
	"github.com/jekabolt/slf"
	
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/types/eth"
)

const erc20TransferName = "transfer(address,uint256)"
// Topic of ERC20/721 `Transfer(address,address,uint256)` event.
const transferEventTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"


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

type TransactionReceipt struct {
	Status    bool
	TokenTransfers []TokenTransfer
	DeployedContract eth.Address
}

type TokenTransfer struct {
	Contract  eth.Address
	From      eth.Address
	To        eth.Address
	Value     eth.Amount
	Removed   bool // TODO: shall we keep this?
}

func minInt(a, b int) int {
	if a < b {
		return a
	}

	return b
}

func getDataArguments(data string, numberOfArgs int) ([]string, error) {
	// Parse arguments from hex-encoded data string, each argument is
	// expected to be 64-char wide, which corresponds to 32-byte (uint256)
	// alignment of arguments in smart contract call protocol.
	// Data may have an "0x" prefix

	if strings.HasPrefix(data, "0x") {
		data = data[2:]
	}

	if len(data) < numberOfArgs * 64 {
		return []string{}, errors.Errorf(
			"Not enough data for %d arguments.", numberOfArgs)
	}

	arguments := []string{}
	for i := 0; i < numberOfArgs; i++ {
		start := i * 64
		end := (i + 1) * 64

		arguments = append(arguments, data[start:end])
	}

	return arguments, nil
}

func getEventLogArguments(log ethrpc.Log, numberOfArgs int) ([]string, error) {
	// Some smart contracts put all arguments to the topics, other put 
	// only portion of arguments as topics, rest as data.
	// So let's assume that if there are less topics than expected arguments,
	// then remaining arguments are in the data.

	arguments := []string{}

	for i := 0; i < minInt(len(log.Topics), numberOfArgs); i++ {
		arguments = append(arguments, log.Topics[i])
	}

	if len(arguments) < numberOfArgs {
		dataArguments, err := getDataArguments(log.Data, numberOfArgs - len(arguments))
		if err != nil {
			return []string{}, err
		}

		arguments = append(arguments, dataArguments...)
	}

	return arguments, nil
}

func (client *Client) getTransactionReceipt(transactionHash string) (result *TransactionReceipt, err error) {
	log := log.WithField("txid", transactionHash)
	receipt, err := client.Rpc.EthGetTransactionReceipt(transactionHash)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch transaction receipt from Node; %+v", err)
	}
	if len(receipt.BlockHash) == 0 && len(receipt.TransactionHash) == 0 {
		// Since only recent recepts are available on node, empty receipt is not an error.
		return nil, nil
	}

	result = &TransactionReceipt{
		Status: bool(receipt.Status != 0),
	}
	if !result.Status {
		return result, nil
	}

	// A sentinel to verify that we parse all receipts properly
	defer func() {
		originalReceipt := fmt.Sprintf("%#v", receipt)
		if len(result.TokenTransfers) == 0 && strings.Contains(originalReceipt, transferEventTopic[2:]) {
			log.Fatalf("\n\n%[1]sTransaction receipt contains transfer log that was not parsed correctly.%[1]s\n\noriginal %#v\nparsed: %#v\n\n\n\n",
				"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n",
				originalReceipt,
				result)
		}
	}()

	for i, logEntry := range receipt.Logs {
		// log := log.WithField("Log #", i)
		if len(logEntry.Topics) >= 1 && logEntry.Topics[0] == transferEventTopic {
			var tokenTransfer TokenTransfer

			eventArguments, err := getEventLogArguments(logEntry, 4)
			if err != nil {
				return nil, err
			}
			// log.Debugf("!!!!\t GOT A TRANSFER EVENT on : %#v", logEntry)

			tokenTransfer.Contract = eth.HexToAddress(logEntry.Address)
			if err != nil {
				return nil, errors.Wrapf(err, "Log %d: failed read 'Address' as contract address", i)
			}

			tokenTransfer.From = eth.HexToAddress(eventArguments[1])
			if err != nil {
				return nil, errors.Wrapf(err, "Log %d: failed read 'From' address", i)
			}

			tokenTransfer.To = eth.HexToAddress(eventArguments[2])
			if err != nil {
				return nil, errors.Wrapf(err, "Log %d: failed read 'To' address", i)
			}

			tokenTransfer.Value, err = eth.HexToAmount(eventArguments[3])
			if err != nil {
				return nil, errors.Wrapf(err, "Log %d: failed to read amount", i)
			}

			tokenTransfer.Removed = logEntry.Removed
			result.TokenTransfers = append(result.TokenTransfers, tokenTransfer)
		}
	}

	return result, nil
}


// TODO: provide eth.BlockHeader instead of blockHeight to use block time form node.
func (client *Client) parseETHTransaction(rawTX ethrpc.Transaction, blockHeight int64, isResync bool) {
	log := log.WithFields(slf.Fields{"txid": rawTX.Hash, "blockHeight": blockHeight, "resync": isResync})
	var fromUser string
	var toUser string

	if udFrom, ok := client.UsersData.Load(rawTX.From); ok {
		fromUser = udFrom.(string)
	}

	if udTo, ok := client.UsersData.Load(rawTX.To); ok {
		toUser = udTo.(string)
	}

	var transactionReceipt *TransactionReceipt
	var err error
	if blockHeight > 0 {
		// Only makes sence to fetch receipt for transactions that are included in any block.
		transactionReceipt, err = client.getTransactionReceipt(rawTX.Hash)
		// log.Debugf("\treceipt: %#v", transactionReceipt)
		if err != nil {
			log.Errorf("Failed to get transacion receipt: %+v", err)
		}
	}

	if toUser == "" && fromUser == "" {
		ignoreTransaction := true

		callInfo, err := DecodeSmartContractCall(rawTX.Input)
		if callInfo != nil {
			log.Infof("Smart contract call info: %v, err: %v", callInfo, err)
		}

		if callInfo != nil && callInfo.Name == erc20TransferName {
			address := callInfo.Arguments[0].(eth.Address)
			if udTokenTo, ok := client.UsersData.Load(address.Hex()); ok {
				ignoreTransaction = false
				log.Debugf("!!! TOKEN TX for tracked user %+v", udTokenTo)
			}
		}

		// Check if token was transferred to or from our user.
		if transactionReceipt != nil && len(transactionReceipt.TokenTransfers) > 0 {
			for _, t := range transactionReceipt.TokenTransfers {
				if _, ok := client.UsersData.Load(t.From); ok {
					ignoreTransaction = false;
					break;
				}
				if _, ok := client.UsersData.Load(t.To); ok {
					ignoreTransaction = false;
					break;
				}
			}
		}

		if ignoreTransaction {
			// not our users tx
			return
		}
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
