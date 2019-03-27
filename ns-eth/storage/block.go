package storage

import (
	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	eth "github.com/Multy-io/Multy-back/common/eth"
)

// Stores blocks in DB and provides convinient access to those
// Typical operations:
// * Save block
// * Remove block by id
// * Read block by id
// * Set/Get immutable block id
// * get all blocks from given id to immutable block

const (
	lastSeenBlockDocumentId = "lastSeenBlock"
)

type BlockStorage struct {
	collection *mgo.Collection
}

func NewBlockStorage(collection *mgo.Collection) (*BlockStorage, error) {
	result := &BlockStorage {
		collection: collection,
	}

	err := collection.EnsureIndex(mgo.Index{
		Key:    []string{"hash"},
		Unique: true,
	})
	if err != nil {
		return nil, reportError(result, err, "Failed to create index.")
	}

	return result, nil
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
	err := self.collection.Remove(bson.M{"hash":blockId})
	if err != nil {
		return reportError(self, err, "delete block failed")
	}

	return nil
}

func (self *BlockStorage) GetBlock(blockId eth.BlockHash) (*eth.Block, error) {
	block := eth.Block{}
	err := self.collection.Find(bson.M{"hash":blockId}).One(&block)
	if err != nil {
		return nil, reportError(self, err, "read block failed")
	}

	return &block, nil
}

func (self *BlockStorage) SetLastSeenBlock(blockHash eth.BlockHash) error {
	_, err := self.collection.UpsertId(lastSeenBlockDocumentId, bson.M{"last_seen_block_hash": blockHash})

	return reportError(self, err, "write last seen block hash failed")
}

func (self *BlockStorage) GetLastSeenBlock() (eth.BlockHash, error) {
	result := eth.BlockHash{}

	var doc bson.M
	err := self.collection.FindId(lastSeenBlockDocumentId).One(&doc)
	if err != nil {
		return result, reportError(self, err, "read last seen block failed")
	}

	result.SetBytes(doc["last_seen_block_hash"].([]byte))
	return result, nil
}