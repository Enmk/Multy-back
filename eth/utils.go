/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package eth

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Multy-io/Multy-back/currencies"
	ethpb "github.com/Multy-io/Multy-back/ns-eth-protobuf"
	"github.com/Multy-io/Multy-back/store"
	nsq "github.com/bitly/go-nsq"
	_ "github.com/jekabolt/slflog"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	exRate    *mgo.Collection
	usersData *mgo.Collection

	txsData *mgo.Collection
	// multisigData *mgo.Collection

	// txsDataTest *mgo.Collection
	// multisigDataTest *mgo.Collection

	restoreState *mgo.Collection
)

func updateWalletAndAddressDate(tx store.TransactionETH, networkID int) error {

	sel := bson.M{"userID": tx.UserID, "wallets.addresses.address": tx.From}
	user := store.User{}
	err := usersData.Find(sel).One(&user)
	update := bson.M{}

	var ok bool

	for i := range user.Wallets {
		for j, addr := range user.Wallets[i].Adresses {
			if addr.Address == tx.From && user.Wallets[i].NetworkID == networkID {
				ok = true
				update = bson.M{
					"$set": bson.M{
						"wallets." + strconv.Itoa(i) + ".lastActionTime":                                   time.Now().Unix(),
						"wallets." + strconv.Itoa(i) + ".addresses." + strconv.Itoa(j) + ".lastActionTime": time.Now().Unix(),
						"wallets." + strconv.Itoa(i) + ".status":                                           store.WalletStatusOK,
					},
				}
				break
			}
		}
	}

	if ok {
		err = usersData.Update(sel, update)
		if err != nil {
			return errors.New("updateWalletAndAddressDate:usersData.Update: " + err.Error())
		}
	}

	sel = bson.M{"userID": tx.UserID, "wallets.addresses.address": tx.To}
	user = store.User{}
	err = usersData.Find(sel).One(&user)
	update = bson.M{}

	ok = false

	for i := range user.Wallets {
		for j, addr := range user.Wallets[i].Adresses {
			if addr.Address == tx.To && user.Wallets[i].NetworkID == networkID {
				ok = true
				update = bson.M{
					"$set": bson.M{
						"wallets." + strconv.Itoa(i) + ".lastActionTime":                                   time.Now().Unix(),
						"wallets." + strconv.Itoa(i) + ".addresses." + strconv.Itoa(j) + ".lastActionTime": time.Now().Unix(),
						"wallets." + strconv.Itoa(i) + ".status":                                           store.WalletStatusOK,
					},
				}
				break
			}
		}
	}
	if ok {
		err = usersData.Update(sel, update)
		if err != nil {
			return errors.New("updateWalletAndAddressDate:usersData.Update: " + err.Error())
		}

	}

	return nil
}

func GetReSyncExchangeRate(time int64) ([]store.ExchangeRatesRecord, error) {
	selCCCAGG := bson.M{
		"stockexchange": "CCCAGG",
		"timestamp":     bson.M{"$lt": time},
	}
	stocksCCCAGG := store.ExchangeRatesRecord{}
	err := exRate.Find(selCCCAGG).Sort("-timestamp").One(&stocksCCCAGG)
	return []store.ExchangeRatesRecord{stocksCCCAGG}, err
}

func GetLatestExchangeRate() ([]store.ExchangeRatesRecord, error) {
	selGdax := bson.M{
		"stockexchange": "Gdax",
	}
	selPoloniex := bson.M{
		"stockexchange": "Poloniex",
	}
	stocksGdax := store.ExchangeRatesRecord{}
	err := exRate.Find(selGdax).Sort("-timestamp").One(&stocksGdax)
	if err != nil {
		return nil, err
	}

	stocksPoloniex := store.ExchangeRatesRecord{}
	err = exRate.Find(selPoloniex).Sort("-timestamp").One(&stocksPoloniex)
	if err != nil {
		return nil, err
	}
	return []store.ExchangeRatesRecord{stocksPoloniex, stocksGdax}, nil
}

func setExchangeRates(tx *store.TransactionETH, isReSync bool, TxTime int64) {
	var err error
	if isReSync {
		rates, err := GetReSyncExchangeRate(tx.BlockTime)
		if err != nil {
			log.Errorf("processTransaction:ExchangeRates: %s", err.Error())
		}
		tx.StockExchangeRate = rates
		return
	}
	if !isReSync || err != nil {
		rates, err := GetLatestExchangeRate()
		if err != nil {
			log.Errorf("processTransaction:ExchangeRates: %s", err.Error())
		}
		tx.StockExchangeRate = rates
	}
}

// TODO: refactor this method where rewrite notify user about transaction
func sendNotifyToClients(tx store.TransactionETH, nsqProducer *nsq.Producer, netid int, userid ...string) error {
	//TODO: make correct notify

	if tx.Status == store.TxStatusAppearedInBlockIncoming || tx.Status == store.TxStatusAppearedInMempoolIncoming || tx.Status == store.TxStatusInBlockConfirmedIncoming {
		txMsq := store.TransactionWithUserID{
			UserID: tx.UserID,
			NotificationMsg: &store.WsTxNotify{
				CurrencyID:      currencies.Ether,
				NetworkID:       netid,
				Address:         tx.To,
				Amount:          tx.Amount,
				TxID:            tx.Hash,
				TransactionType: tx.Status,
				WalletIndex:     tx.WalletIndex,
				From:            tx.From,
				To:              tx.To,
				//	Multisig:        tx.Multisig.Contract,
			},
		}
		if len(userid) > 0 {
			txMsq.UserID = userid[0]
		}
		return sendNotify(&txMsq, nsqProducer)
	}

	if tx.Status == store.TxStatusAppearedInBlockOutcoming || tx.Status == store.TxStatusAppearedInMempoolOutcoming || tx.Status == store.TxStatusInBlockConfirmedOutcoming {
		txMsq := store.TransactionWithUserID{
			UserID: tx.UserID,
			NotificationMsg: &store.WsTxNotify{
				CurrencyID:      currencies.Ether,
				NetworkID:       netid,
				Address:         tx.From,
				Amount:          tx.Amount,
				TxID:            tx.Hash,
				TransactionType: tx.Status,
				WalletIndex:     tx.WalletIndex,
				From:            tx.From,
				To:              tx.To,
				//	Multisig:        tx.Multisig.Contract,
			},
		}
		return sendNotify(&txMsq, nsqProducer)
	}

	log.Errorf("!!! TX %s NOTIFICATION WAS NOT SENT TO USER (unknown tx status: %d)", tx.Hash, tx.Status)
	return nil
}

func sendNotify(txMsq *store.TransactionWithUserID, nsqProducer *nsq.Producer) error {
	newTxJSON, err := json.Marshal(txMsq)
	if err != nil {
		log.Errorf("sendNotifyToClients: [%+v] %s\n", txMsq, err.Error())
		return err
	}
	err = nsqProducer.Publish(store.TopicTransaction, newTxJSON)
	if err != nil {
		log.Errorf("nsq publish new transaction: [%+v] %s\n", txMsq, err.Error())
		return err
	}

	return nil
}

func generatedTxDataToStore(tx *ethpb.ETHTransaction, TransactionStatus int) store.TransactionETH {
	sel := bson.M{"wallets.addresses.address": tx.GetFrom()}
	user := store.User{}
	usersData.Find(sel).One(&user)
	var walletIndex, addressIndex int
	for _, wallet := range user.Wallets {
		for _, address := range wallet.Adresses {
			if tx.GetFrom() == address.Address {
				walletIndex = wallet.WalletIndex
				addressIndex = address.AddressIndex
			}
		}
	}

	return store.TransactionETH{
		UserID:       user.UserID,  // tx.GetUserID(),
		WalletIndex:  walletIndex,  //int(tx.GetWalletIndex()),
		AddressIndex: addressIndex, // int(tx.GetAddressIndex()),
		Hash:         tx.GetHash(),
		From:         tx.GetFrom(),
		To:           tx.GetTo(),
		Amount:       tx.GetAmount(),
		GasPrice:     tx.GetGasPrice(),
		GasLimit:     tx.GetGasLimit(),
		Nonce:        tx.GetNonce(),
		Status:       TransactionStatus, // int(tx.GetStatus()),
		BlockTime:    tx.GetBlockTime(),
		PoolTime:     tx.GetTxpoolTime(),
		BlockHeight:  tx.GetBlockHeight(),
	}
}

func saveTransaction(tx store.TransactionETH, networtkID int, resync bool) error {

	txStore := &mgo.Collection{}
	switch networtkID {
	case currencies.ETHMain:
		txStore = txsData
	// case currencies.ETHTest:
	// 	txStore = txsDataTest
	default:
		return errors.New("saveMultyTransaction: wrong networkID")
	}

	// This is splited transaction! That means that transaction's WalletsInputs and WalletsOutput have the same WalletIndex!
	//Here we have outgoing transaction for exact wallet!
	multyTX := store.TransactionETH{}
	if tx.Status == store.TxStatusAppearedInBlockIncoming || tx.Status == store.TxStatusAppearedInMempoolIncoming || tx.Status == store.TxStatusInBlockConfirmedIncoming {
		log.Debugf("saveTransaction new incoming tx to %v", tx.To)
		sel := bson.M{"userid": tx.UserID, "hash": tx.Hash, "walletindex": tx.WalletIndex}
		err := txStore.Find(sel).One(&multyTX)
		if err == mgo.ErrNotFound {
			// initial insertion
			err := txStore.Insert(tx)
			return err
		}
		if err != nil && err != mgo.ErrNotFound {
			// database error
			return err
		}

		update := bson.M{
			"$set": bson.M{
				"txstatus":    tx.Status,
				"blockheight": tx.BlockHeight,
				"blocktime":   tx.BlockTime,
				"gasprice":    tx.GasPrice,
				"gaslimit":    tx.GasLimit,
			},
		}
		err = txStore.Update(sel, update)
		return err
	} else if tx.Status == store.TxStatusAppearedInBlockOutcoming || tx.Status == store.TxStatusAppearedInMempoolOutcoming || tx.Status == store.TxStatusInBlockConfirmedOutcoming {
		log.Debugf("saveTransaction new outcoming tx  %v", tx.From)
		sel := bson.M{"userid": tx.UserID, "hash": tx.Hash, "walletindex": tx.WalletIndex}
		err := txStore.Find(sel).One(&multyTX)
		if err == mgo.ErrNotFound {
			// initial insertion
			err := txStore.Insert(tx)
			return err
		}
		if err != nil && err != mgo.ErrNotFound {
			// database error
			return err
		}

		update := bson.M{
			"$set": bson.M{
				"txstatus":    tx.Status,
				"blockheight": tx.BlockHeight,
				"blocktime":   tx.BlockTime,
				"gasprice":    tx.GasPrice,
				"gaslimit":    tx.GasLimit,
			},
		}
		err = txStore.Update(sel, update)
		if err != nil {
			log.Errorf("saveMultyTransaction:txsData.Update %s", err.Error())
		}
		return err
	}
	return nil
}

func FetchUserAddresses(currencyID, networkID int, user store.User, addreses []string) ([]store.AddressExtended, error) {
	addresses := []store.AddressExtended{}
	fetched := map[string]store.AddressExtended{}

	for _, address := range addreses {
		fetched[address] = store.AddressExtended{
			Address:    address,
			Associated: false,
		}
	}

	for _, wallet := range user.Wallets {
		for _, addres := range wallet.Adresses {
			if wallet.CurrencyID == currencyID && wallet.NetworkID == networkID {
				for addr, fetchAddr := range fetched {
					if addr == addres.Address {
						fetchAddr.Associated = true
						fetchAddr.WalletIndex = wallet.WalletIndex
						fetchAddr.AddressIndex = addres.AddressIndex
						fetchAddr.UserID = user.UserID
						fetched[addres.Address] = fetchAddr
					}
				}
			}
		}
	}

	for _, addr := range fetched {
		addresses = append(addresses, addr)
	}

	return addresses, nil
}

func fetchMethod(input string) string {
	method := input
	if len(input) < 10 {
		method = "0x"
	} else {
		method = input[:10]
	}

	return method
}

func parseSubmitInput(input string) (string, string) {
	address := ""
	amount := ""
	// 266 is minimal length of valid input for this kind of transactions
	if len(input) >= 266 {
		// crop method name from input data
		in := input[10:]
		re := regexp.MustCompile(`.{64}`) // Every 64 chars
		parts := re.FindAllString(in, -1) // Split the string into 64 chars blocks.

		// 4 is minimal count of parts for correct method invocation
		if len(parts) >= 4 {
			address = strings.ToLower("0x" + parts[0][24:])
			a, _ := new(big.Int).SetString(parts[1], 16)
			amount = a.String()
		}
	}

	return address, amount
}

func parseRevokeInput(input string) (int64, error) {
	in := input[10:]
	re := regexp.MustCompile(`.{64}`) // Every 64 chars
	parts := re.FindAllString(in, -1) // Split the string into 64 chars blocks.

	if len(parts) > 0 {
		i, ok := new(big.Int).SetString(input, 16)
		if !ok {
			return 0, fmt.Errorf("bad input %v", input)
		} else {
			return i.Int64(), nil
		}

	}

	return 0, fmt.Errorf("low len input %v", input)
}

func msToUserData(addresses []string, usersData *mgo.Collection) map[string]store.User {
	users := map[string]store.User{} // ms attached address to user
	for _, address := range addresses {
		user := store.User{}
		err := usersData.Find(bson.M{"wallets.addresses.address": strings.ToLower(address)}).One(&user)
		if err != nil {
			break
		}
		// attachedAddress = strings.ToLower(address)
		users[strings.ToLower(address)] = user
	}
	return users
}
