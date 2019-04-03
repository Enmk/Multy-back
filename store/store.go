/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package store

import (
	"fmt"
	"strconv"
	"time"

	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/currencies"
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	errType        = errors.New("wrong database type")
	errEmplyConfig = errors.New("empty configuration for datastore")
)

// Default table names
const (
	TableUsers             = "UserCollection"
	TableStockExchangeRate = "TableStockExchangeRate"
)

// Conf is a struct for database configuration
type Conf struct {
	Address             string
	DBUsers             string
	DBFeeRates          string
	DBTx                string
	DBStockExchangeRate string

	// BTC main
	TableTxsDataBTCMain          string
	TableSpendableOutputsBTCMain string
	TableSpentOutputsBTCMain     string

	// BTC test
	TableMempoolRatesBTCTest     string
	TableTxsDataBTCTest          string
	TableSpendableOutputsBTCTest string
	TableSpentOutputsBTCTest     string

	// ETH main
	TableMultisigTxsMain string
	TableTxsDataETHMain  string

	// ETH main
	TableMultisigTxsTest string
	TableTxsDataETHTest  string

	//RestoreState
	DBRestoreState string
	TableState     string

	//Authentification
	Username string
	Password string
}

type QualifiedAddress struct {
	UserID string
	WalletIndex int
	AddressIndex int
}

type UserStore interface {
	GetUserByDevice(device bson.M, user *User) error
	Update(sel, update bson.M) error
	Insert(user User) error
	Close() error
	FindUser(query bson.M, user *User) error
	UpdateUser(sel bson.M, user *User) error
	// FindUserTxs(query bson.M, userTxs *TxRecord) error
	// InsertTxStore(userTxs TxRecord) error
	FindUserErr(query bson.M) error
	FindUserAddresses(query bson.M, sel bson.M, ws *WalletsSelect) error
	InsertExchangeRate(ExchangeRates, string) error
	GetExchangeRatesDay() ([]RatesAPIBitstamp, error)

	FindAllUserAddresses(address string) ([]QualifiedAddress, error)

	//TODo update this method by eth
	GetAllWalletEthTransactions(userid string, currencyID, networkID int, walletTxs *[]TransactionETH) error

	DeleteWallet(userid, address string, walletindex, currencyID, networkID, assetType int) error

	FindAllUserETHTransactions(sel bson.M) ([]TransactionETH, error)
	FindUserDataChain(CurrencyID, NetworkID int) (map[string]AddressExtended, error)

	FetchUserAddresses(currencyID, networkID int, userid string, addreses []string) (AddressExtended, error)

	DeleteHistory(CurrencyID, NetworkID int, Address string) error
	ConvertToBroken(addresses []string, userid string)

	FetchLastSyncBlockState(networkid, currencyid int) (int64, error)

	CheckAddWallet(wp *WalletParams, jwt string) error

	CheckTx(tx string) bool
}

type MongoUserStore struct {
	config    *Conf
	session   *mgo.Session
	usersData *mgo.Collection

	//eth main
	// ETHMainRatesData *mgo.Collection
	ETHMainTxsData *mgo.Collection

	//eth test
	// ETHTestRatesData *mgo.Collection
	ETHTestTxsData *mgo.Collection

	stockExchangeRate *mgo.Collection
	ethTxHistory      *mgo.Collection
	ETHTest           *mgo.Collection

	RestoreState *mgo.Collection
}

func InitUserStore(conf Conf) (UserStore, error) {
	uStore := &MongoUserStore{
		config: &conf,
	}

	addr := []string{conf.Address}

	mongoDBDial := &mgo.DialInfo{
		Addrs:    addr,
		Username: conf.Username,
		Password: conf.Password,
		Timeout:  1 * time.Second,
	}

	session, err := mgo.DialWithInfo(mongoDBDial)
	if err != nil {
		return nil, err
	}

	// HACK: this made to acknowledge that queried data has already inserted to db
	session.SetSafe(&mgo.Safe{
		W:        1,
		WTimeout: 100,
		J:        true,
	})

	uStore.session = session
	uStore.usersData = uStore.session.DB(conf.DBUsers).C(TableUsers)
	uStore.stockExchangeRate = uStore.session.DB(conf.DBStockExchangeRate).C(TableStockExchangeRate)

	// ETH main
	uStore.ETHMainTxsData = uStore.session.DB(conf.DBTx).C(conf.TableTxsDataETHMain)

	// ETH test
	uStore.ETHTestTxsData = uStore.session.DB(conf.DBTx).C(conf.TableTxsDataETHTest)

	uStore.RestoreState = uStore.session.DB(conf.DBRestoreState).C(conf.TableState)

	return uStore, nil
}

func (mStore *MongoUserStore) CheckTx(tx string) bool {
	query := bson.M{"txid": tx}
	// sp := SpendableOutputs{}
	err := mStore.usersData.Find(query).One(nil)
	if err != nil {
		return true
	}
	return false
}

func (mStore *MongoUserStore) FindUserDataChain(CurrencyID, NetworkID int) (map[string]AddressExtended, error) {
	users := []User{}
	usersData := map[string]AddressExtended{} // addres -> userid
	err := mStore.usersData.Find(nil).All(&users)
	if err != nil {
		return usersData, err
	}
	for _, user := range users {
		for _, wallet := range user.Wallets {
			if wallet.CurrencyID == CurrencyID && wallet.NetworkID == NetworkID {
				for _, address := range wallet.Adresses {
					usersData[address.Address] = AddressExtended{
						UserID:       user.UserID,
						WalletIndex:  wallet.WalletIndex,
						AddressIndex: address.AddressIndex,
					}
				}
			}
		}
	}
	return usersData, nil
}

func (mStore *MongoUserStore) FetchUserAddresses(currencyID, networkID int, userid string, addreses []string) (AddressExtended, error) {
	user := User{}
	err := mStore.usersData.Find(bson.M{"userID": userid}).One(&user)
	if err != nil {
		return AddressExtended{}, err
	}
	addresses := []AddressExtended{}

	for _, wallet := range user.Wallets {
		for _, addres := range wallet.Adresses {
			if wallet.CurrencyID == currencyID && wallet.NetworkID == networkID {
				for _, fetchAddr := range addreses {

					ae := AddressExtended{
						Address:    fetchAddr,
						Associated: false,
					}
					if fetchAddr == addres.Address {
						ae.Associated = true
						ae.WalletIndex = wallet.WalletIndex
						ae.AddressIndex = addres.AddressIndex
						ae.UserID = userid
					}
					addresses = append(addresses, ae)
				}

			}
		}
	}
	return AddressExtended{}, nil
}

func (mStore *MongoUserStore) ConvertToBroken(addresses []string, userid string) {
	for _, address := range addresses {
		sel := bson.M{"userID": userid, "wallets.addresses.address": address}
		fmt.Println(sel)
		update := bson.M{"$set": bson.M{
			"wallets.$.brokenStatus": 1,
		}}
		err := mStore.usersData.Update(sel, update)
		fmt.Println("err : ", err)
	}
}

func (mStore *MongoUserStore) DeleteHistory(CurrencyID, NetworkID int, Address string) error {

	switch CurrencyID {
	case currencies.Ether:
		if NetworkID == currencies.ETHMain {

		}
		if NetworkID == currencies.ETHTest {

		}
	}
	return nil
}

func (mStore *MongoUserStore) FetchLastSyncBlockState(networkid, currencyid int) (int64, error) {
	ls := LastState{}
	sel := bson.M{"networkid": networkid, "currencyid": currencyid}
	err := mStore.RestoreState.Find(sel).One(&ls)
	return ls.BlockHeight, err
}

func (mStore *MongoUserStore) FindAllUserETHTransactions(sel bson.M) ([]TransactionETH, error) {
	allTxs := []TransactionETH{}
	err := mStore.ethTxHistory.Find(sel).All(&allTxs)
	return allTxs, err
}
func (mStore *MongoUserStore) FindETHTransaction(sel bson.M) error {
	err := mStore.ethTxHistory.Find(sel).One(nil)
	return err
}

func (mStore *MongoUserStore) DeleteWallet(userid, address string, walletindex, currencyID, networkID, assetType int) error {
	var err error
	switch assetType {
	case AssetTypeMultyAddress:
		user := User{}
		sel := bson.M{"userID": userid, "wallets.networkID": networkID, "wallets.currencyID": currencyID, "wallets.walletIndex": walletindex}
		err = mStore.usersData.Find(bson.M{"userID": userid}).One(&user)
		var position int
		if err == nil {
			for i, wallet := range user.Wallets {
				if wallet.NetworkID == networkID && wallet.WalletIndex == walletindex && wallet.CurrencyID == currencyID {
					position = i
					break
				}
			}
			update := bson.M{
				"$set": bson.M{
					"wallets." + strconv.Itoa(position) + ".status": WalletStatusDeleted,
				},
			}
			return mStore.usersData.Update(sel, update)
		}
	case AssetTypeImportedAddress:
		query := bson.M{"userID": userid, "wallets.addresses.address": address, "wallets.isImported": true}
		update := bson.M{
			"$set": bson.M{
				"wallets.$.status": WalletStatusDeleted,
			},
		}
		err := mStore.usersData.Update(query, update)
		if err != nil {
			return errors.New("DeleteWallet:restClient.userStore.Update:AssetTypeImportedAddress " + err.Error())
		}
		return err
	case AssetTypeMultisig:

	}

	return err

}

func (mStore *MongoUserStore) UpdateUser(sel bson.M, user *User) error {
	return mStore.usersData.Update(sel, user)
}

func (mStore *MongoUserStore) GetUserByDevice(device bson.M, user *User) error { // rename GetUserByToken
	return mStore.usersData.Find(device).One(user)
}

func (mStore *MongoUserStore) Update(sel, update bson.M) error {
	return mStore.usersData.Update(sel, update)
}

func (mStore *MongoUserStore) FindUser(query bson.M, user *User) error {
	return mStore.usersData.Find(query).One(user)
}
func (mStore *MongoUserStore) FindUserErr(query bson.M) error {
	return mStore.usersData.Find(query).One(nil)
}

func (mStore *MongoUserStore) FindUserAddresses(query bson.M, sel bson.M, ws *WalletsSelect) error {
	return mStore.usersData.Find(query).Select(sel).One(ws)
}

func (mStore *MongoUserStore) Insert(user User) error {
	return mStore.usersData.Insert(user)
}

func (mStore *MongoUserStore) InsertExchangeRate(eRate ExchangeRates, exchangeStock string) error {
	eRateRecord := &ExchangeRatesRecord{
		Exchanges:     eRate,
		Timestamp:     time.Now().Unix(),
		StockExchange: exchangeStock,
	}

	return mStore.stockExchangeRate.Insert(eRateRecord)
}

// GetExchangeRatesDay returns exchange rates for last day with time interval equal to hour
func (mStore *MongoUserStore) GetExchangeRatesDay() ([]RatesAPIBitstamp, error) {
	// not implemented
	return nil, nil
}

func (mStore *MongoUserStore) GetAllWalletEthTransactions(userid string, currencyID, networkID int, walletTxs *[]TransactionETH) error {
	switch currencyID {
	case currencies.Ether:
		query := bson.M{"userid": userid}
		if networkID == currencies.ETHMain {
			err := mStore.ETHMainTxsData.Find(query).All(walletTxs)
			return err
		}
		if networkID == currencies.ETHTest {
			err := mStore.ETHTestTxsData.Find(query).All(walletTxs)
			return err
		}

	}
	return nil
}

func (mStore *MongoUserStore) Close() error {
	mStore.session.Close()
	return nil
}

func (mStore *MongoUserStore) CheckAddWallet(wp *WalletParams, jwt string) error {
	user := User{}
	query := bson.M{"devices.JWT": jwt}
	err := mStore.usersData.Find(query).One(&user)
	if err != nil {
		return err
	}

	count := 0
	for _, wallet := range user.Wallets {
		if wallet.CurrencyID == wp.CurrencyID && wallet.NetworkID == wp.NetworkID {
			count++
		}
	}
	if count < MaximumAvalibeEmptyWallets {
		return nil
	}

	if count >= MaximumAvalibeEmptyWallets {
		query := bson.M{"userid": user.UserID}
		switch wp.CurrencyID {
		case currencies.Ether:
			txs := []TransactionETH{}
			switch wp.NetworkID {
			case currencies.ETHMain:
				err = mStore.ETHMainTxsData.Find(query).All(&txs)
			// case currencies.ETHTest:
			// 	err = mStore.ETHTestTxsData.Find(query).All(&txs)
			}
			if len(txs) == 0 {
				return errors.Errorf("Reached maximum available wallets count")
			}
		}
	}

	return err
}

func (store *MongoUserStore) FindAllUserAddresses(address string) ([]QualifiedAddress, error) {

	query := bson.M{"wallets.addresses.address": address}
	iter := store.usersData.Find(query).Iter()
	defer iter.Close()

	result := make([]QualifiedAddress, 0, 10)

	// Alternative would be using projections and pipes, but that is too much rocket-science for now.
	var user User
	for iter.Next(&user) {
		for _, w := range user.Wallets {
			for _, a := range w.Adresses {
				if a.Address == address {
					result = append(result, QualifiedAddress{
						UserID: user.UserID,
						WalletIndex: w.WalletIndex,
						AddressIndex: a.AddressIndex,
					})
				}
			}
		}

		err := iter.Err()
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to fetch qualified addresses from DB.")
		}
	}

	return result, nil
}