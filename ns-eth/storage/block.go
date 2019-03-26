package storage

import (
	mgo "gopkg.in/mgo.v2"
	// "gopkg.in/mgo.v2/bson"

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

func (self *BlockStorage) SetLastSeenBlockHeader(blockHeader eth.BlockHeader) error {
	_, err := self.collection.UpsertId(lastSeenBlockDocumentId, blockHeader)

	return reportError(self, err, "write last seen block id failed")
}

func (self *BlockStorage) GetLastSeenBlockHeader() (*eth.BlockHeader, error) {
	result := eth.BlockHeader{}
	err := self.collection.FindId(lastSeenBlockDocumentId).One(&result)
	if err != nil {
		return nil, reportError(self, err, "read last seen block id failed")
	}

	return &result, nil
}