package storage

import (
	"gopkg.in/mgo.v2/bson"
	mgo "gopkg.in/mgo.v2"

	eth "github.com/Multy-io/Multy-back/types/eth"
)

type TransactionStorage struct {
	transactionCollection *mgo.Collection
}

func NewTransactionStorage(transactionCollection *mgo.Collection) *TransactionStorage {
	return &TransactionStorage {
		transactionCollection,
	}
}

func (self *TransactionStorage) getErrorContext() string {
	return self.transactionCollection.FullName
}

func (self *TransactionStorage) LoadTransaction(transactionId eth.TransactionHash) (*eth.TransactionWithStatus, error) {
	result := eth.TransactionWithStatus{}
	err := self.transactionCollection.FindId(transactionId).One(&result)

	return nil, reportError(self, err, "read transaction failed")
}

func (self *TransactionStorage) SaveTransaction(transaction eth.TransactionWithStatus) error {
	_, err := self.transactionCollection.UpsertId(transaction.ID, transaction);

	return reportError(self, err, "write transaction failed")
}

func (self *TransactionStorage) UpdateTransactionStatus(transactionId eth.TransactionHash, newStatus eth.TransactionStatus) error {
	err := self.transactionCollection.UpdateId(transactionId, bson.M{"status": newStatus})

	return reportError(self, err, "transaction status update failed")
}


