/*
Copyright 2019 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"context"
	"fmt"
	"math/big"

	"github.com/pkg/errors"

	pb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/common/eth"
	"github.com/Multy-io/Multy-back/store"
	// "github.com/Multy-io/Multy-back/ns-eth/storage"
)

func (ethcli *EthController) setGRPCHandlers(networkID int, accuracyRange int) {

	// TODO: Write method  transaction handler with NSQ from ns-eth

	// TODO: update blockHeight on database and check logic parce and send notify from ns-eth
	// TODO: move logic rejected tx  to ns-eth look on old commit with it logic

	// watch for channel and push to node
	go func() {
		for addr := range ethcli.WatchAddress {
			// TODO: split chan and move emit new address to nsq
			err := ethcli.eventManager.EmitNewAddressEvent(eth.HexToAddress(addr.Address))
			if err != nil {
				log.Errorf("NewAddressNode: cli.EventAddNewAddress %s\n", err.Error())
			}
			// log.Debugf("EventAddNewррррнAddress Reply %s", rp)

			rp, err := ethcli.GRPCClient.ResyncAddress(context.Background(), &pb.Address{
				Address: addr.Address,
			})
			if err != nil {
				log.Errorf("EventResyncAddress: cli.EventResyncAddress %s\n", err.Error())
			}
			log.Debugf("EventResyncAddress Reply %s", rp)
		}
	}()

}

func (controller *EthController) HandleTransactionStatus(txStatusEvent eth.TransactionStatusEvent) error {
	// Happy path:
	// Load TX from DB (by TX HASH)
	// if not in DB or BLOCK HASH differs from one in DB:
	//    get from NS
	//    save to DB
	// split into user transactions:
	//    one tx per known user address (and for token transfers too)
	// save each split transaction to user transactions DB, with BLOCK HASH + TX HASH as reference to corresponding blockchain TX
	// ???? handle error states and replaced transactions ????

	transaction, err := controller.fetchAndUpdateTransaction(txStatusEvent)
	if err != nil || transaction == nil {
		if err == nil {
			err = fmt.Errorf("Failed to fetch corresponding transaction for %s", txStatusEvent.TransactionHash)
		}
		return err
	}

	userTransactions, err := controller.splitTransactionToUserTransactions(transaction)
	log.Debugf("Split transaction %s into %#v with error: %+v", txStatusEvent.TransactionHash, userTransactions, err)
	// TODO: save all transactions, update wallets balances and notify all corresponding users.

	// Save every transaction to the DB and send notifications to clients.
	for _, tx := range userTransactions {
		log.WithField("tx", fmt.Sprintf("[%s (%s => %s | %s)]", tx.Hash, tx.From, tx.To, tx.Token))
		err := saveTransaction(tx, controller.coinType.NetworkID, false)
		if err != nil {
			log.Debugf("!!! Failed to save tx with err: %+v", err)
		}
		err = sendNotifyToClients(tx, controller.FirebaseNsqProducer, controller.coinType.NetworkID, tx.UserID)
		if err != nil {
			log.Debugf("!!! Failed to send tx notification to user: %+v", err)
		}
	}

	return nil
}

func (controller *EthController) fetchAndUpdateTransaction(txStatusEvent eth.TransactionStatusEvent) (*eth.TransactionWithStatus, error) {
	// Return most up-to-date version of the transaction based on local cache and recent version on node service.
	// If we need to get transaction from NS, we store the updated version to DB.
	// Since transaction result depends on the context (i.e. the block it is in),
	// we track changed block hash as a reason to reload transaction from NS.

	// TODO: handle partial transaction updates:
	// - cached transaction has events, but transaction from ns does not (since receipt is no longer available)

	cachedTransaction, err := controller.transactionStorage.GetTransaction(txStatusEvent.TransactionHash)
	if err != nil || cachedTransaction == nil {
		// Don't care about errors, since we can re-write transaction to DB later.
		log.Infof("Faield to load transaction from DB: %#v, %+v", cachedTransaction, err)
	}

	if cachedTransaction != nil {
		if blockInfo := cachedTransaction.BlockInfo; blockInfo != nil {
			if blockInfo.Hash != txStatusEvent.BlockHash {
				// transaction exist in cache, but status update event tells us that it
				// was included in another block.
				// Refetch it from node-service, since new block means different blockchain state
				// and basically could change transaction side-effects.
				cachedTransaction = nil
			}
		}
	}

	if cachedTransaction == nil {
		// Fetch the most up-to date transaction from node-service,
		// and cache it to the DB.
		newTransactionPb, err := controller.GRPCClient.GetTransaction(
			context.TODO(),
			&pb.TransactionHash{Hash: txStatusEvent.TransactionHash.Hex()})

		if err != nil || newTransactionPb == nil {
			if err != nil {
				err = fmt.Errorf("Transaction is NIL")
			}
			return nil, errors.Wrapf(err, "Failed to get transaction from node-service by hash %s: %+v", txStatusEvent.TransactionHash.Hex(), err)
		}

		cachedTransactionNoStatus, err := pb.TransactionFromProtobuf(*newTransactionPb)
		if err != nil || cachedTransactionNoStatus == nil {
			if err == nil {
				err = fmt.Errorf("Transaction is NIL")
			}
			return nil, errors.Wrapf(err, "Failed to convert transaction from protobuf.")
		}

		cachedTransaction = &eth.TransactionWithStatus{
			Transaction: *cachedTransactionNoStatus,
			Status: txStatusEvent.Status,
		}

		err = controller.transactionStorage.AddTransaction(*cachedTransaction)
		if err != nil {
			return nil, errors.WithMessage(err, "Failed to save transaction to DB")
		}
	}

	return cachedTransaction, nil
}

// transferDescriptor describes transfer of value from sender to recepient
type transferDescriptor struct {
	from   eth.Address
	to     eth.Address
	token  eth.Address // smart contract address or empty for ETH
	value  string      // erc20: value transferred, erc721: id of token
}

type transferDirection int
const transferIncoming transferDirection = +1
const transferOutgoing transferDirection = +1

func (controller *EthController) splitTransactionToUserTransactions(transaction *eth.TransactionWithStatus) ([]store.TransactionETH, error) {

	// single transaction may cause several transfers of value,
	// to exclude duplicates we arrange all those transfers in a 'set'
	// in a way that transfer(a, b, c) call and corresponding Transfer(a, b, c) event on ERC20 token
	// have same descriptor, and hence do not produce duplicate transactions.
	descriptors := make(map[transferDescriptor]struct{})
	dummy := struct{}{}
	descriptors[transferDescriptor{
		transaction.Sender,
		transaction.Receiver,
		eth.Address{},
		transaction.Amount.Int.Text(10)}] = dummy

	if callInfo := transaction.CallInfo; callInfo != nil {
		d, err := controller.getTransferDescriptor(callInfo.Method)
		if err != nil || d == nil {
			log.Infof("can't get transaction descriptor for method call: %+v", err)
		}
		if d != nil {
			descriptors[*d] = dummy
		}

		for i, event := range callInfo.Events {
			d, err := controller.getTransferDescriptor(&event)
			if err != nil || d == nil {
				log.Infof("can't get transaction descriptor for event #%d: %+v", i, err)
				continue
			}

			descriptors[*d] = dummy
		}
	}

	// Has information common to all split transactions.
	templateTransaction := store.TransactionETH{
		Hash:       transaction.Hash.Hex(),
		From:       transaction.Sender.Hex(),
		To:         transaction.Receiver.Hex(),
		Amount:     transaction.Amount.Hex(),
		GasPrice:   uint64(transaction.Fee.GasLimit),
		GasLimit:   uint64(transaction.Fee.GasPrice),
		Nonce:      uint64(transaction.Nonce),
		// Status:     int(transaction.Status),
		// PoolTime
	}
	if blockInfo := transaction.BlockInfo; blockInfo != nil {
		templateTransaction.BlockTime = blockInfo.Time.Unix()
		templateTransaction.BlockHeight = int64(blockInfo.Height)
		templateTransaction.BlockHash = blockInfo.Hash.Hex()
	}

	result := make([]store.TransactionETH, 0, len(descriptors) * 2) // at max 2 user-transactions per transfer: 1 incoming 1 outgoing

	// For each party in descriptors, find all user/wallet/address tuples, and make corresponding transactions.
	for d := range descriptors {

		transfers := []struct{
			address   eth.Address
			direction transferDirection
		}{{d.from, transferOutgoing}, {d.to, transferIncoming}}

		for _, transfer := range transfers {
			qualifiedAddresses, err := controller.userStore.FindAllUserAddresses(transfer.address.Hex())
			if err != nil {
				return nil, errors.WithMessagef(err, "Failed to fetch qualified addresses for: %s", transfer.address)
			}

			for _, addr := range qualifiedAddresses {
				tx := templateTransaction

				tx.UserID = addr.UserID
				tx.WalletIndex = addr.WalletIndex
				tx.AddressIndex = addr.AddressIndex
				tx.Status = convertStatus(transaction.Status, transfer.direction)

				result = append(result, tx)
			}
		}
	}

	return result, nil
}

func (controller *EthController) getTransferDescriptor(call *eth.SmartContractMethodInfo) (*transferDescriptor, error) {
	if call == nil {
		return nil, nil
	}

	if !controller.isSupportedContractAddress(call.Address) {
		return nil, errors.Errorf("Unsuported contract address %s for method call %s", call.Address.Hex(), call.Name)
	}

	if call.Name == eth.Erc20TransferName || call.Name == eth.TransferEventName {
		var arguments struct {
			Sender   eth.Address
			Receiver eth.Address
			Amount   big.Int
		}
		err := call.UnpackArguments(&arguments)
		if err != nil {
			return nil, err
		}

		return &transferDescriptor{
			from:  arguments.Sender,
			to:    arguments.Receiver,
			token: call.Address,
			value: arguments.Amount.Text(16),
		}, nil
	}

	return nil, errors.Errorf("Unknown call signature %s", call.Name)
}

func convertStatus(status eth.TransactionStatus, direction transferDirection) int {
	var statuses map[eth.TransactionStatus]int

	if direction == transferIncoming {
		statuses = map[eth.TransactionStatus]int{
			eth.TransactionStatusInMempool:        store.TxStatusAppearedInMempoolIncoming,
			eth.TransactionStatusInBlock:          store.TxStatusAppearedInBlockIncoming,
			eth.TransactionStatusInImmutableBlock: store.TxStatusInBlockConfirmedIncoming,
			eth.TransactionStatusErrorRejected:    store.TxStatusTxRejectedIncoming,
			eth.TransactionStatusErrorSmartContractCallFailed: store.TxStatusInBlockMethodInvocationFail,
			//eth.TransactionStatusErrorReplaced: ????
			//eth,TransactionStatusErrorLost: ????
		}
	} else if direction == transferOutgoing {
		statuses = map[eth.TransactionStatus]int{
			eth.TransactionStatusInMempool:        store.TxStatusAppearedInMempoolOutcoming,
			eth.TransactionStatusInBlock:          store.TxStatusAppearedInBlockOutcoming,
			eth.TransactionStatusInImmutableBlock: store.TxStatusInBlockConfirmedOutcoming,
			eth.TransactionStatusErrorRejected:    store.TxStatusTxRejectedOutgoing,
			eth.TransactionStatusErrorSmartContractCallFailed: store.TxStatusInBlockMethodInvocationFail,
			//eth.TransactionStatusErrorReplaced: ????
			//eth,TransactionStatusErrorLost: ????
		}
	} else {
		panic(errors.Errorf("Unknown transaction direction: %d", direction))
	}

	result, ok := statuses[status]
	if !ok {
		panic(errors.Errorf("Don't know how to convert status %d with direction %d", status, direction))
	}

	return result
}

func (controller *EthController) isSupportedContractAddress(address eth.Address) bool {
	// TODO: check with whitelist of tokens
	return true
}

func (controller *EthController) HandleBlock(block eth.BlockHeader) error {
	log.Infof("!!!!! ON NEW BLOCK: %#v", block)
	return nil
}