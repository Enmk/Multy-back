package storage

import (
	mgo "gopkg.in/mgo.v2"
	"time"
)

const (
	addressCollectionName = "addresses"
	blockCollectionName = "blocks"
	transactionCollectionName = "transactions"
)

type Storage struct {
	db *mgo.Database

	AddressStorage		*AddressStorage
	BlockStorage		*BlockStorage
	TransactionStorage	*TransactionStorage
}

type Config struct {
	URL      string
	Password string
	Username string
	Database string
	Timeout  time.Duration
}

func (self *Storage) getErrorContext() string {
	return self.db.Name
}

func NewStorage(config Config) (*Storage, error) {
	mongoDBDial := mgo.DialInfo{
		Addrs:		[]string{config.URL},
		Username:	config.Username,
		Password:	config.Password,
		Timeout:	config.Timeout,
	}

	dbSession, err := mgo.DialWithInfo(&mongoDBDial)
	if err != nil {
		return nil, err
	}

	dbSession.SetSafe(&mgo.Safe{
		W:        1,
		WTimeout: 100,
		J:        true,
	})

	db := dbSession.DB(config.Database)
	blockStorage, err := NewBlockStorage(db.C(blockCollectionName))
	if err != nil {
		return nil, err
	}

	return &Storage{
		db,
		NewAddressStorage(db.C(addressCollectionName)),
		blockStorage,
		NewTransactionStorage(db.C(transactionCollectionName)),
	}, nil
}

func (self *Storage) Close() {
	self.db.Session.Close()
}