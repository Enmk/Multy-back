
package storage

import (
	"sync"
	"gopkg.in/mgo.v2/bson"
	mgo "gopkg.in/mgo.v2"

	eth "github.com/Multy-io/Multy-back/types/eth"
)

// Stores all eth addresses that we filter transactions against
// each address is a separate (empty) document.

type addressSet map[string]struct{}

func toKey(address eth.Address) string {
	return string(address.Bytes())
}
type AddressStorage struct {
	collection *mgo.Collection

	m sync.RWMutex
	cachedAddresses addressSet
}

func NewAddressStorage(collection *mgo.Collection) *AddressStorage {
	return &AddressStorage {
		collection: collection,
		cachedAddresses: make(addressSet),
	}
}

func (self *AddressStorage) getErrorContext() string {
	return self.collection.FullName
}

func (self *AddressStorage) AddAddress(newAddress eth.Address) error {
	self.m.Lock()
	defer self.m.Unlock()

	_, err := self.collection.UpsertId(newAddress, bson.M{"_id": newAddress.Bytes()})
	if err != nil {
		return reportError(self, err, "adding address failed")
	}

	self.cachedAddresses[toKey(newAddress)] = struct{}{}
	return nil
}

func (self *AddressStorage) IsAddressExists(address eth.Address) bool {
	// 	count, err := self.collection.FindId(address).Count()
	// 	if err != nil {
	// 		return reportError(self, err, "reading address failed")
	
	// 		return false
	// 	}

	// 	return count > 0
	
	// }

	self.m.RLock()
	defer self.m.RUnlock()

	_, exists := self.cachedAddresses[toKey(address)]

	return exists
}

func (self *AddressStorage) LoadAllAddresses() error {
	// TODO: do not lock, but rather, add a flag that would signal cache is Ok, if it is not, IsAddressExists() should read from DB.
	self.m.Lock()
	defer self.m.Unlock()

	newAddresses := make(addressSet)

	iter := self.collection.Find(nil).Iter()
	defer iter.Close()

	var addressDoc bson.M
	for iter.Next(&addressDoc) {
		newAddresses[toKey(addressDoc["_id"].(eth.Address))] = struct{}{}
	}

	err := iter.Err()
	if err != nil && err != mgo.ErrNotFound {
		return reportError(self, err, "reading all addresses failed")
	}

	self.cachedAddresses = newAddresses

	return nil
}