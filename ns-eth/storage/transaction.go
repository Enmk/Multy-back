package storage

import (
	"gopkg.in/mgo.v2/bson"
	mgo "gopkg.in/mgo.v2"

	eth "github.com/Multy-io/Multy-back/common/eth"
)

type TransactionStorage struct {
	collection *mgo.Collection
}

func NewTransactionStorage(collection *mgo.Collection) *TransactionStorage {
	return &TransactionStorage {
		collection,
	}
}

func (self *TransactionStorage) getErrorContext() string {
	return self.collection.FullName
}

func (self *TransactionStorage) GetTransaction(transactionId eth.TransactionHash) (*eth.TransactionWithStatus, error) {
	result := &eth.TransactionWithStatus{}
	err := self.collection.FindId(transactionId).One(result)
	if err != nil {
		return nil, reportError(self, err, "read transaction failed")
	}

	return result, nil
}

func (self *TransactionStorage) AddTransaction(transaction eth.TransactionWithStatus) error {
	_, err := self.collection.UpsertId(transaction.Hash, &transaction);

	return reportError(self, err, "write transaction failed")
}

func (self *TransactionStorage) UpdateTransactionStatus(transactionId eth.TransactionHash, newStatus eth.TransactionStatus) error {
	err := self.collection.UpdateId(transactionId, bson.M{"$set": bson.M{"status": newStatus}})

	return reportError(self, err, "transaction status update failed")
}

// UpdateManyTransactionsStatus updates multiple transactions in DB, returning number of TX updated or error.
// func (self *TransactionStorage) UpdateManyTransactionsStatus(transactionIds []eth.TransactionHash, newStatus eth.TransactionStatus) (int, error) {
// 	query := bson.M{"_id": bson.M{"$in": transactionIds}}
// 	info, err := self.collection.UpdateAll(query, bson.M{"status": newStatus})
// 	if err != nil {
// 		return 0, reportError(self, err, "multiple transaction status update failed")
// 	}

// 	return info.Updated, nil
// }

func (self *TransactionStorage) GetTransactionStatus(transactionId eth.TransactionHash) (eth.TransactionStatus, error) {
	result := &eth.TransactionWithStatus{}
	err := self.collection.FindId(transactionId).Select(bson.M{"status": 1}).One(&result)

	return eth.TransactionStatus(result.Status), reportError(self, err, "transaction status read failed")
}

func (self *TransactionStorage) RemoveTransaction(transactionId eth.TransactionHash) error {
	err := self.collection.RemoveId(transactionId)
	if err != nil {
		return reportError(self, err, "transaction remove failed")
	}

	return nil
}