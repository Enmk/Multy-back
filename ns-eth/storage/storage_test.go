package storage

import (
	"testing"

	"os"
	"log"
	"reflect"
	"time"

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
		Timeout:  time.Millisecond * 100,
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
				Amount: eth.NewAmountFromInt64(1337),
				Nonce: 42,
				Fee : eth.TransactionFee{
					GasLimit: 10000,
					GasPrice: 100*eth.GWei,
				},
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

	if !reflect.DeepEqual(*actualBlock, expectedBlock) {
		test.Fatalf("block : expected(%+v) != actual(%+v)", expectedBlock, actualBlock)
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

	actualImmutableBlockID, err := storage.BlockStorage.GetImmutableBlockId()
	if err != nil {
		test.Fatalf("failed to get immutable block")
	}

	if expectedImmutableBlockID != *actualImmutableBlockID {
		test.Fatalf("immutable block id: expected(%+v) != actual(%+v)", expectedImmutableBlockID, actualImmutableBlockID)
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
	storage := newEmptyStorage()
	defer storage.Close()

	err := storage.AddressStorage.AddAddress("mockaddress")
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok := storage.AddressStorage.IsAddressExists("mockaddress")
	if ok != true {
		test.Fatalf("failed to find already added address")
	}
}

func TestTransactionStorage(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const txID eth.TransactionHash = "mock transaction hash"

	err := storage.TransactionStorage.RemoveTransaction(txID)
	if err != nil {
		test.Fatalf("Removing transaction from empty database failed: %+v", err)
	}

	tx, err := storage.TransactionStorage.GetTransaction(txID)
	if err == nil {
		test.Fatalf("Loading transaction from empty database expected to fail, got transaction: %+v", tx)
	}

	expectedTx := eth.TransactionWithStatus{
		Transaction: eth.Transaction{
			ID: txID,
			Sender: "sender",
			Receiver: "receiver",
			Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			Amount: eth.NewAmountFromInt64(1337),
			Nonce: 42,
			Fee : eth.TransactionFee{
				GasLimit: 10000,
				GasPrice: 100*eth.GWei,
			},
		},
		Status: eth.TransactionStatusInMempool,
	}

	err = storage.TransactionStorage.AddTransaction(expectedTx)
	if err != nil {
		test.Fatalf("failed to save transaction to the DB: %+v", err)
	}

	err = storage.TransactionStorage.AddTransaction(expectedTx)
	if err != nil {
		test.Fatalf("failed to save transaction to the DB second time: %+v", err)
	}

	actualTx, err := storage.TransactionStorage.GetTransaction(expectedTx.ID)
	if err != nil {
		test.Fatalf("Failed to load existing transaction from DB: %+v", err)
	}

	if !reflect.DeepEqual(expectedTx, *actualTx) {
		test.Fatalf("transaction : expected(%+v) != actual(%+v)", expectedTx, actualTx)
	}
}

func TestTransactionStorageTransactionStatus(test *testing.T) {
	storage := newEmptyStorage()
	defer storage.Close()

	const txID eth.TransactionHash = "mock transaction hash"

	expectedTx := eth.TransactionWithStatus{
		Transaction: eth.Transaction{
			ID: txID,
			Sender: "sender",
			Receiver: "receiver",
			Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			Amount: eth.NewAmountFromInt64(1337),
			Nonce: 42,
			Fee : eth.TransactionFee{
				GasLimit: 10000,
				GasPrice: 100*eth.GWei,
			},
		},
		Status: eth.TransactionStatusInMempool,
	}

	err := storage.TransactionStorage.UpdateTransactionStatus(txID, eth.TransactionStatusError)
	if err == nil {
		test.Fatal("transaction status update on non-existing transaction expected to fail")
	}

	err = storage.TransactionStorage.AddTransaction(expectedTx)
	if err != nil {
		test.Fatalf("failed to save transaction to the DB: %+v", err)
	}

	err = storage.TransactionStorage.UpdateTransactionStatus(txID, eth.TransactionStatusError)
	if err != nil {
		test.Fatalf("failed to update transaction status: %+v", err)
	}
}