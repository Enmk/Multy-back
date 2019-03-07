package storage

import (
	"testing"

	"os"
	"log"
	"math/big"
	"reflect"

	mgo "gopkg.in/mgo.v2"
	eth "github.com/Multy-io/Multy-back/types/eth"
)

var (
	config Config
)

func newEmptyStorage() *Storage {
	storage, err := NewStorage(config)
	if err != nil {
		log.Fatalf("Failed to connect to the MongoDB instance with %v : %v", config, err)
	}

	dbName := storage.db.Name
	// Dropping a DB in order to cleanup all collections
	err = storage.db.DropDatabase()
	if err != nil {
		log.Fatalf("Failed to drop database: %s", dbName)
	}

	return storage
}

func TestMain(m *testing.M) {
	config = Config{
		Address: os.Getenv("MONGO_DB_ADDRESS"),
		Username: os.Getenv("MONGO_DB_USER"),
		Password: os.Getenv("MONGO_DB_PASSWORD"),
		Database: os.Getenv("MONGO_DB_DATABASE_NS_STORE"),
	}

	if os.Getenv("DGAMING_BACK_VERBOSE_TESTS") != "" || os.Getenv("DGAMING_BACK_VERBOSE_TESTS_MONGO") != "" {
		var aLogger *log.Logger
		aLogger = log.New(os.Stderr, "| mgo | ", log.LstdFlags)
		mgo.SetLogger(aLogger)
		mgo.SetDebug(true)
	}

	_, err := NewStorage(config)
	if err != nil {
		log.Fatalf("Failed to connect to the MongoDB instance with %v : %v", config, err)
	}

	os.Exit(m.Run())
}

func TestBlockStorage(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const blockID = "mock block id"
	expectedBlock := eth.Block{
		BlockHeader: eth.BlockHeader{
			ID: blockID,
			Height: 10,
			Parent: "mock block parent",
		},
		Transactions: []eth.Transaction{
			{
				ID: "mock transaction id",
				Sender: "sender",
				Receiver: "receiver",
				Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
				Amount: big.NewInt(1337),
				Nonce: 42,
			},
		},
	}
	err := storage.BlockStorage.AddBlock(expectedBlock)
	if err != nil {
		test.Fatalf("failed to add a block: %+v", err)
	}

	actualBlock, err := storage.BlockStorage.GetBlock(blockID)
	if err != nil {
		test.Fatalf("failed to get block: %+v", err)
	}

	if reflect.DeepEqual(*actualBlock, expectedBlock) {
		test.Fatalf("block read from store is different from expected: (%+v) != (%+v)", actualBlock, expectedBlock)
	}

	err = storage.BlockStorage.RemoveBlock(blockID)
	if err != nil {
		test.Fatalf("failed to delete block: %+v", err)
	}
}

func TestBlockStorageRemoveNonExisting(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	err := storage.BlockStorage.RemoveBlock("mock block id")
	if err != nil {
		test.Fatalf("failed to delete non-existing block")
	}
}

func TestBlockStorageImmutableBlock(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const expectedImmutableBlockID = "immutable block id"
	err := storage.BlockStorage.SetImmutableBlockId(expectedImmutableBlockID)
	if err != nil {
		test.Fatalf("failed to set immutable block: %+v", err)
	}

	actualImmutableBlockId, err := storage.BlockStorage.GetImmutableBlockId()
	if err != nil {
		test.Fatalf("failed to get immutable block")
	}

	if expectedImmutableBlockID != *actualImmutableBlockId {
		test.Fatalf("immutable block %+v != %+v", expectedImmutableBlockID, actualImmutableBlockId)
	}
}

func TestAddressStorage(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const address = "mockaddress"

	ok := storage.AddressStorage.IsAddressExists(address)
	if ok != false {
		test.Fatalf("found address that does not exist yet")
	}

	err := storage.AddressStorage.AddAddress(address)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok = storage.AddressStorage.IsAddressExists(address)
	if ok != true {
		test.Fatalf("failed to find already added address")
	}
}

func TestAddressStorageLoadAllAddressesTwice(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	err := storage.AddressStorage.LoadAllAddresses()
	if err != nil {
	 	test.Fatalf("failed to load all addresses: %+v", err)
	}

	err = storage.AddressStorage.LoadAllAddresses()
	if err != nil {
	 	test.Fatalf("failed to load all addresses second time: %+v", err)
	}
}

func TestAddressStorageAddAddressTwice(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const address = "mockaddress"

	err := storage.AddressStorage.AddAddress(address)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	err = storage.AddressStorage.AddAddress(address)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok := storage.AddressStorage.IsAddressExists(address)
	if ok != true {
		test.Fatalf("failed to find already added address")
	}

	count, err := storage.AddressStorage.addressCollection.Count()
	if err != nil {
		test.Fatalf("failed to count documents")
	}

	if count != 1 {
		test.Fatalf("Expected 1 document in collection, found: %d", count)
	}
}

func TestAddressStorageAddAddress(test *testing.T) {
	storage, err := NewStorage(config)

	if err != nil {
		test.Fatalf("failed to build a new Store instance: %+v", err)
	}

	// Do not load addresses from DB first

	err = storage.AddressStorage.AddAddress("mockaddress")
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok := storage.AddressStorage.IsAddressExists("mockaddress")
	if ok != true {
		test.Fatalf("failed to find already added address")
	}

	storage.Close()
}