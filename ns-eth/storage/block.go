package storage

import (
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	eth "github.com/Multy-io/Multy-back/types/eth"
)

// Stores blocks in DB and provides convinient access to those
// Typical operations:
// * Save block
// * Remove block by id
// * Read block by id
// * Set/Get immutable block id
// * get all blocks from given id to immutable block

const (
	immutableBlockDocumentId = "immutableBlock"
)

type BlockStorage struct {
	collection *mgo.Collection
}

func NewBlockStorage(collection *mgo.Collection) *BlockStorage {
	return &BlockStorage {
		collection: collection,
	}
}

func (self *BlockStorage) getErrorContext() string {
	return self.collection.FullName
}

func (self *BlockStorage) AddBlock(newBlock eth.Block) error {
	err := self.collection.Insert(&newBlock)
	if err != nil {
		return reportError(self, err, "write block failed")
	}

	return nil
}

func (self *BlockStorage) RemoveBlock(blockId eth.BlockHash) error {
	err := self.collection.RemoveId(blockId)
	if err != nil {
		return reportError(self, err, "delete block failed")
	}

	return nil
}

func (self *BlockStorage) GetBlock(blockId eth.BlockHash) (*eth.Block, error) {
	block := eth.Block{}
	err := self.collection.FindId(blockId).One(&block)
	if err != nil {
		return nil, reportError(self, err, "read block failed")
	}

	return &block, nil
}

func (self *BlockStorage) SetImmutableBlockId(imutableBlockId eth.BlockHash) error {
	_, err := self.collection.UpsertId(immutableBlockDocumentId, bson.M{"immutable_block": imutableBlockId})

	return reportError(self, err, "write immutable block id failed")
}

func (self *BlockStorage) GetImmutableBlockId() (*eth.BlockHash, error) {
	var immutableBlockDoc bson.M
	err := self.collection.FindId(immutableBlockDocumentId).One(&immutableBlockDoc)
	if err != nil {
		return nil, reportError(self, err, "read immutable block id failed")
	}

	blockHash := (eth.BlockHash)(immutableBlockDoc["immutable_block"].(string))
	return &blockHash, nil
}