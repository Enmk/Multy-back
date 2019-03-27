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
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/davecgh/go-spew/spew"
	"github.com/jekabolt/slf"
	
	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/common/eth"
)

const erc20TransferName = "transfer(address,uint256)"
// Signature of ERC20/721 `Transfer` event.
const transferEventName = "Transfer(address,address,uint256)"

const transferEventTopic = "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"


func (client *NodeClient) SendRawTransaction(rawTX string) (string, error) {
	hash, err := client.Rpc.EthSendRawTransaction(rawTX)
	if err != nil {
		log.Errorf("SendRawTransaction:rpc.EthSendRawTransaction: %s", err.Error())
		return hash, errors.Wrapf(err, "Failed to send raw transaction")
	}
	return hash, err
}

func (client *NodeClient) GetAddressBalance(address string) (big.Int, error) {
	balance, err := client.Rpc.EthGetBalance(address, "latest")
	if err != nil {
		log.Errorf("GetAddressBalance:rpc.EthGetBalance: %s", err.Error())
		return balance, err
	}
	return balance, err
}

func (client *NodeClient) GetTxByHash(hash string) (bool, error) {
	tx, err := client.Rpc.EthGetTransactionByHash(hash)
	if tx == nil {
		return false, err
	} else {
		return true, err
	}
}

func (client *NodeClient) GetAddressPendingBalance(address string) (big.Int, error) {
	balance, err := client.Rpc.EthGetBalance(address, "pending")
	if err != nil {
		log.Errorf("GetAddressPendingBalance:rpc.EthGetBalance: %s", err.Error())
		return balance, err
	}
	log.Debugf("GetAddressPendingBalance %v", balance.String())
	return balance, err
}

func (client *NodeClient) GetAllTxPool() ([]byte, error) {
	return client.Rpc.TxPoolContent()
}

func (client *NodeClient) GetBlockHeight() (int, error) {
	return client.Rpc.EthBlockNumber()
}

func (client *NodeClient) GetCode(address string) (string, error) {
	return client.Rpc.EthGetCode(address, "latest")
}

func (client *NodeClient) GetAddressNonce(address string) (big.Int, error) {
	return client.Rpc.EthGetTransactionCount(address, "latest")
}

func (client *NodeClient) ResyncTransaction(txid string) error {
	tx, err := client.Rpc.EthGetTransactionByHash(txid)
	if err != nil {
		return err
	}
	if tx != nil {
		client.HandleEthTransaction(*tx, tx.BlockNumber, true)
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

func (client *NodeClient) fetchTransactionCallInfo(rawTx ethrpc.Transaction) (*eth.SmartContractCallInfo, error) {
	receipt, err := client.Rpc.EthGetTransactionReceipt(rawTx.Hash)
	if receipt == nil || err != nil {
		return nil, errors.Wrapf(err, "Failed to fetch transaction receipt from Node; %+v", err)
	}

	result, err := decodeTransactionCallInfo(rawTx, receipt)

	originalReceipt := fmt.Sprintf("%#v", receipt)
	// A sentinel to verify that we parse all receipts properly
	if result != nil && len(result.Events) == 0 && strings.Contains(originalReceipt, transferEventTopic[2:]) {
		log.Fatalf("\n\n%[1]s!!!!!!! Transaction receipt contains transfer log that was not parsed correctly !!!!!!!\n%[1]s\n\nreceipt: %s\nparsed: %s\n\ntx: %s\n\n",
			"!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!\n",
			spew.Sdump(receipt),
			spew.Sdump(result),
			spew.Sdump(rawTx))
	}

	return result, err
}

func decodeTransactionCallInfo(rawTx ethrpc.Transaction, receipt *ethrpc.TransactionReceipt) (*eth.SmartContractCallInfo, error) {
	log := log.WithField("txid", rawTx.Hash)

	if receipt == nil || (len(receipt.BlockHash) == 0 && len(receipt.TransactionHash) == 0) {
		log.Infof("Got empty (or nil) receipt from server: %#+v", receipt)
		// Since only recent recepts are available on node, empty receipt is not an error.
		return nil, nil
	}

	var methodInfo *eth.SmartContractMethodInfo
	var err error
	if len(rawTx.Input) > smartContractCallSigSize * 2 {
		methodInfo, err = DecodeSmartContractCall(rawTx.Input, eth.HexToAddress(rawTx.To))
		if _, ok := err.(ABIError); !ok && err != nil {
			log.Debugf("Failed to decode smart contract method call: %#v", err)
			// NOTE: it is Ok if call can't be decoded, Input could be not a SC call or unknown method call.
		}
	}

	var deployedAddress *eth.Address
	if receipt.ContractAddress != "" {
		address := eth.HexToAddress(receipt.ContractAddress)
		deployedAddress = &address
	}

	events := make([]eth.SmartContractEventInfo, 0, len(receipt.Logs))
	for i, logEntry := range receipt.Logs {
		logData := strings.Join(logEntry.Topics, "") + logEntry.Data
		logData = "0x" + strings.ReplaceAll(logData, "0x", "")
		event, err := DecodeSmartContractEvent(logData, eth.HexToAddress(logEntry.Address))
		if _, ok := err.(ABIError); !ok && err != nil {
			log.Infof("Failed to decode transaction log #%d: %v", i, err)
		}
		if event != nil {
			events = append(events, *event)
		}
	}

	return &eth.SmartContractCallInfo{
		Status: eth.SmartContractCallStatus(receipt.Status),
		Method: methodInfo,
		DeployedAddress: deployedAddress,
		Events: events,
	}, nil
}

func (client *NodeClient) fetchTransactionInfo(rawTX ethrpc.Transaction, block *eth.Block) (*eth.Transaction, error) {
	log := log.WithField("txid", rawTX.Hash)

	var err error
	var callInfo *eth.SmartContractCallInfo

	var payload []byte
	if len(rawTX.Input) > 0 {
		payload, err = hexutil.Decode(rawTX.Input)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to decode transaction payload from hex: %s.", rawTX.Input)
		}
	}

	if rawTX.BlockNumber > 0 {
		callInfo, err = client.fetchTransactionCallInfo(rawTX)
		if err != nil {
			log.Errorf("Failed to get transacion call info: %+v", err)
			return nil, errors.WithMessage(err, "from fetchAllTransactionInfo")
		}
	}

	var blockInfo *eth.TransactionBlockInfo
	if block != nil {
		blockInfo = &eth.TransactionBlockInfo{
			Hash:   block.BlockHeader.Hash,
			Height: block.BlockHeader.Height,
			Time:   block.BlockHeader.Time,
		}
	}

	return &eth.Transaction{
		Hash:     eth.HexToHash(rawTX.Hash),
		Sender:   eth.HexToAddress(rawTX.From),
		Receiver: eth.HexToAddress(rawTX.To),
		Payload:  payload,
		Amount:   eth.Amount{rawTX.Value},
		Nonce:    eth.TransactionNonce(rawTX.Nonce),
		Fee: eth.TransactionFee{
			GasPrice: eth.GasPrice(rawTX.GasPrice.Uint64()),
			GasLimit: eth.GasLimit(rawTX.Gas),
		},
		CallInfo:  callInfo,
		BlockInfo: blockInfo,
	}, nil

	// if useTransaction == false {

	// 	callInfo, err := DecodeSmartContractCall(rawTX.Input)
	// 	if callInfo != nil {
	// 		log.Infof("Smart contract call info: %v, err: %v", callInfo, err)
	// 	}

	// 	if callInfo != nil && callInfo.Name == erc20TransferName {
	// 		address, ok := callInfo.Arguments[0].(eth.Address)
	// 		if ok == true {
	// 			useTransaction = client.IsAnyKnownAddress(address)
	// 		} else {
	// 			log.Errorf("Unexpected argument 0 type for erc20 `transfer()`: %#v, expected Address", callInfo)
	// 		}
	// 	}

	// 	// Check if token was transferred to or from our user.
	// 	if transactionReceipt != nil && len(transactionReceipt.TokenTransfers) > 0 {
	// 		for _, t := range transactionReceipt.TokenTransfers {
	// 			if client.IsAnyKnownAddress(t.From, t.To) == true {
	// 				useTransaction = true
	// 				break;
	// 			}
	// 		}
	// 	}

	// 	if useTransaction == false {
	// 		// not our users tx
	// 		return
	// 	}
	// }

	// tx := rawToGenerated(rawTX)
	// tx.Resync = isResync

	// block, err := client.Rpc.EthGetBlockByHash(rawTX.BlockHash, false)
	// if err != nil {
	// 	if blockHeight == -1 {
	// 		tx.TxpoolTime = time.Now().Unix()
	// 	} else {
	// 		tx.BlockTime = time.Now().Unix()
	// 	}
	// 	tx.BlockHeight = blockHeight
	// } else {
	// 	tx.BlockTime = int64(block.Timestamp)
	// 	tx.BlockHeight = int64(block.Number)
	// }

	// if blockHeight == -1 {
	// 	tx.TxpoolTime = time.Now().Unix()
	// }
}


// TODO: provide eth.BlockHeader instead of blockHeight to use block time form node.
func (client *NodeClient) HandleEthTransaction(rawTX ethrpc.Transaction, blockHeight int64, isResync bool) error {
	log := log.WithFields(slf.Fields{"txid": rawTX.Hash, "blockHeight": blockHeight, "resync": isResync})

	transaction, err := client.fetchTransactionInfo(rawTX, nil)
	if err != nil {
		log.Errorf("fetchTransactionInfo: %#v", err)
		return err
	}

	if client.isTransactionOfKnownAddress(transaction) {
		client.transactionsStream <- *transaction
	}

	return nil

	// // Transaction from known user
	// useTransaction := client.IsAnyKnownAddress(transaction.Sender, transaction.Receiver)

	// var transactionReceipt *TransactionReceipt
	// var err error
	// if blockHeight > 0 {
	// 	// Only makes sence to fetch receipt for transactions that are included in any block.
	// 	transactionReceipt, err = client.getTransactionReceipt(rawTX.Hash)
	// 	// log.Debugf("\treceipt: %#v", transactionReceipt)
	// 	if err != nil {
	// 		log.Errorf("Failed to get transacion receipt: %+v", err)
	// 	}
	// }

	// if useTransaction == false {

	// 	callInfo, err := DecodeSmartContractCall(rawTX.Input)
	// 	if callInfo != nil {
	// 		log.Infof("Smart contract call info: %v, err: %v", callInfo, err)
	// 	}

	// 	if callInfo != nil && callInfo.Name == erc20TransferName {
	// 		address, ok := callInfo.Arguments[0].(eth.Address)
	// 		if ok == true {
	// 			useTransaction = client.IsAnyKnownAddress(address)
	// 		} else {
	// 			log.Errorf("Unexpected argument 0 type for erc20 `transfer()`: %#v, expected Address", callInfo)
	// 		}
	// 	}

	// 	// Check if token was transferred to or from our user.
	// 	if transactionReceipt != nil && len(transactionReceipt.TokenTransfers) > 0 {
	// 		for _, t := range transactionReceipt.TokenTransfers {
	// 			if client.IsAnyKnownAddress(t.From, t.To) == true {
	// 				useTransaction = true
	// 				break;
	// 			}
	// 		}
	// 	}

	// 	if useTransaction == false {
	// 		// not our users tx
	// 		return
	// 	}
	// }

	// tx := rawToGenerated(rawTX)
	// tx.Resync = isResync

	// block, err := client.Rpc.EthGetBlockByHash(rawTX.BlockHash, false)
	// if err != nil {
	// 	if blockHeight == -1 {
	// 		tx.TxpoolTime = time.Now().Unix()
	// 	} else {
	// 		tx.BlockTime = time.Now().Unix()
	// 	}
	// 	tx.BlockHeight = blockHeight
	// } else {
	// 	tx.BlockTime = int64(block.Timestamp)
	// 	tx.BlockHeight = int64(block.Number)
	// }

	// if blockHeight == -1 {
	// 	tx.TxpoolTime = time.Now().Unix()
	// }

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

func (client *NodeClient) IsAnyKnownAddress(addresses ...eth.Address) bool {
	for _, address := range addresses {
		if client.addressLookup.IsAddressExists(address) {
			return true
		}
	}

	return false
}

func (client *NodeClient) isTransactionOfKnownAddress(transaction *eth.Transaction) bool {
	if client.IsAnyKnownAddress(transaction.Sender, transaction.Receiver) {
		return true
	}

	if callInfo := transaction.CallInfo; callInfo != nil {
		if method := callInfo.Method; method != nil {
			if method.Name == erc20TransferName && len(method.Arguments) > 0 {
				address, ok := method.Arguments[0].Value.(eth.Address)
				if ok == true {
					if client.IsAnyKnownAddress(address) {
						return true
					}
				} else {
					log.Errorf("Unexpected argument 0 type for erc20 `transfer()` method: %#v, expected Address", callInfo)
				}
			}
		}

		for _, event := range callInfo.Events {
			if event.Name == transferEventName && len(event.Arguments) >= 3 {
				fromAddress, ok := event.Arguments[0].Value.(eth.Address)
				if ok && client.IsAnyKnownAddress(fromAddress) {
					return true
				}
				toAddress, ok := event.Arguments[1].Value.(eth.Address)
				if ok && client.IsAnyKnownAddress(toAddress) {
					return true
				}
			}
		}
	}

	return false
}