package storage

import (
	"testing"

	"os"
	"log"
	"time"

	mgo "gopkg.in/mgo.v2"
	eth "github.com/Multy-io/Multy-back/common/eth"
	. "github.com/Multy-io/Multy-back/tests"
	. "github.com/Multy-io/Multy-back/tests/eth"
)

var (
	config Config

	// mockTransaction = eth.Transaction{
	// 	ID: mockTransactionId,
	// 	Sender: ToAddress("sender"),
	// 	Receiver: ToAddress("receiver"),
	// 	Payload: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
	// 	Amount: *eth.NewAmountFromInt64(1337),
	// 	Nonce: 42,
	// 	Fee : eth.TransactionFee{
	// 		GasLimit: 10000,
	// 		GasPrice: 100*eth.GWei,
	// 	},
	// }
	mockTransaction = SampleTransaction()

	mockTransactionId = ToTxHash("mock transaction id")
	mockBlockId = ToBlockHash("mock block id")
	mockAddress = ToAddress("mock address")

	mockBlockHeader = eth.BlockHeader{
		ID:     ToBlockHash("mock block hash in header"),
		Height: 1337000,
		Parent: ToBlockHash("mock block hash of parent in header"),
		Time:   time.Now(),
	}
)

func newEmptyStorage(test *testing.T) *Storage {
	c := config
	uniqueDb := GetenvOrDefault("MULTY_TEST_NS_STORAGE_UNINQUE_DB_FOR_EACH_TEST", "0") != "0"
	if uniqueDb {
		c.Database += "_" + test.Name()
		test.Logf("Using unique DB for the test: %s", c.Database)
	}
	storage, err := NewStorage(c)
	if err != nil {
		test.Fatalf("Failed to connect to the MongoDB instance with %#v : %v", config, err)
	}

	dbName := storage.db.Name
	// Dropping a DB in order to cleanup all collections
	err = storage.db.DropDatabase()
	if !uniqueDb && err != nil {
		test.Fatalf("Failed to drop database: %s", dbName)
	}

	return storage
}

func TestMain(m *testing.M) {
	config = Config{
		Address: GetenvOrDefault("MONGO_DB_ADDRESS", "localhost:27017"),
		Username: GetenvOrDefault("MONGO_DB_USER", ""),
		Password: GetenvOrDefault("MONGO_DB_PASSWORD", ""),
		Database: GetenvOrDefault("MONGO_DB_DATABASE_NS_STORE", "ns_test_db"),
		Timeout:  100 * time.Millisecond,
	}

	if os.Getenv("DGAMING_BACK_VERBOSE_TESTS") != "" || os.Getenv("DGAMING_BACK_VERBOSE_TESTS_MONGO") != "" {
		var aLogger *log.Logger
		aLogger = log.New(os.Stderr, "| mgo | ", log.LstdFlags)
		mgo.SetLogger(aLogger)
		mgo.SetDebug(true)
	}

	_, err := NewStorage(config)
	if err != nil {
		log.Fatalf("Failed to connect to the MongoDB instance with %#v : %v", config, err)
	}

	os.Exit(m.Run())
}

func TestBlockStorage(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	expectedBlock := eth.Block{
		BlockHeader: eth.BlockHeader{
			ID: mockBlockId,
			Height: 10,
			Parent: ToBlockHash("mock block parent"),
		},
		Transactions: []eth.Transaction{
			mockTransaction,
		},
	}

	// Write and read block, check if values are the same
	err := storage.BlockStorage.AddBlock(expectedBlock)
	if err != nil {
		test.Fatalf("failed to add a block: %+v", err)
	}

	actualBlock, err := storage.BlockStorage.GetBlock(expectedBlock.ID)
	if err != nil {
		test.Fatalf("failed to get block: %+v", err)
	}

	if equal, l, r := TestEqual(*actualBlock, expectedBlock); !equal {
		test.Fatalf("block: expected != actual\nexpected:\n%s\nactual:\n%s", l, r)
	}

	// Check that RemoveBlock removes
	err = storage.BlockStorage.RemoveBlock(expectedBlock.ID)
	if err != nil {
		test.Fatalf("failed to delete block: %+v", err)
	}
	// Second remove fails
	if _, ok := storage.BlockStorage.RemoveBlock(expectedBlock.ID).(ErrorNotFound); !ok {
		test.Fatalf("Expected storage.ErrorNotFound on already deleted block, got: %+v", err)
	}
	// Get fails after removal
	_, err = storage.BlockStorage.GetBlock(expectedBlock.ID)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("Expected storage.ErrorNotFound on getting non-existing block form DB, got: %+v", err)
	}
}

func TestBlockStorageEmpty(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	// Empty DB, RemoveBlock and GetBlock should return ErrorNotFound
	err := storage.BlockStorage.RemoveBlock(mockBlockId)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("Expected storage.ErrorNotFound on deleting non-existing block, got: %+v", err)
	}

	_, err = storage.BlockStorage.GetBlock(mockBlockId)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("Expected storage.ErrorNotFound on getting non-existing block form DB, got: %+v", err)
	}
}

func TestBlockStorageLastSeenBlock(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	expectedBlockHeader := mockBlockHeader
	err := storage.BlockStorage.SetLastSeenBlockHeader(expectedBlockHeader)
	if err != nil {
		test.Fatalf("failed to set immutable block: %+v", err)
	}

	actualBlockHeader, err := storage.BlockStorage.GetLastSeenBlockHeader()
	if err != nil {
		test.Fatalf("failed to get immutable block")
	}

	if equal, l, r := TestEqual(expectedBlockHeader, *actualBlockHeader); !equal {
		test.Fatalf("immutable block header: expected != actual\nexpected:\n%s\nactual:\n%s", l, r)
	}
}

func TestAddressStorage(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	ok := storage.AddressStorage.IsAddressExists(mockAddress)
	if ok != false {
		test.Fatalf("found address that does not exist yet")
	}

	err := storage.AddressStorage.AddAddress(mockAddress)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok = storage.AddressStorage.IsAddressExists(mockAddress)
	if ok != true {
		test.Fatalf("failed to find already added address")
	}
}

func TestAddressStorageLoadAllAddressesTwice(test *testing.T) {
	storage := newEmptyStorage(test)
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
	storage := newEmptyStorage(test)
	defer storage.Close()

	err := storage.AddressStorage.AddAddress(mockAddress)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	err = storage.AddressStorage.AddAddress(mockAddress)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok := storage.AddressStorage.IsAddressExists(mockAddress)
	if ok != true {
		test.Fatalf("failed to find already added address")
	}

	count, err := storage.AddressStorage.collection.Count()
	if err != nil {
		test.Fatalf("failed to count documents")
	}

	if count != 1 {
		test.Fatalf("Expected 1 document in collection, found: %d", count)
	}
}

func TestAddressStorageAddAddress(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	err := storage.AddressStorage.AddAddress(mockAddress)
	if err != nil {
		test.Fatalf("failed to add address: %+v", err)
	}

	ok := storage.AddressStorage.IsAddressExists(mockAddress)
	if ok != true {
		test.Fatalf("failed to find already added address")
	}
}

func TestTransactionStorage(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	err := storage.TransactionStorage.RemoveTransaction(mockTransactionId)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("expected storage.ErrorNotFound on removing transaction from empty DB, got: %+v", err)
	}

	tx, err := storage.TransactionStorage.GetTransaction(mockTransactionId)
	if err == nil {
		test.Fatalf("Loading transaction from empty database expected to fail, got transaction: %+v", tx)
	}

	expectedTx := eth.TransactionWithStatus{
		Transaction: mockTransaction,
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

	if equal, l, r := TestEqual(expectedTx, *actualTx); !equal {
		test.Fatalf("transaction: expected != actual\nexpected:\n%s\nactual:\n%s", l, r)
	}
}

func TestTransactionStorageTransactionStatus(test *testing.T) {
	storage := newEmptyStorage(test)
	defer storage.Close()

	expectedTx := eth.TransactionWithStatus{
		Transaction: mockTransaction,
		Status: eth.TransactionStatusInMempool,
	}

	// On empty DB: should fail with storage.ErrorNotFound
	err := storage.TransactionStorage.UpdateTransactionStatus(expectedTx.ID, eth.TransactionStatusError)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("expected storage.ErrorNotFound on updating non-existing transaction, got: %+v", err)
	}

	_, err = storage.TransactionStorage.GetTransactionStatus(expectedTx.ID)
	if _, ok := err.(ErrorNotFound); !ok {
		test.Fatalf("expected storage.ErrorNotFound on getting status of non-existing transaction, got: %+v", err)
	}

	// Add TX
	err = storage.TransactionStorage.AddTransaction(expectedTx)
	if err != nil {
		test.Fatalf("failed to save transaction to the DB: %+v", err)
	}

	// Non-empty DB, shouldn't fail now
	status, err := storage.TransactionStorage.GetTransactionStatus(expectedTx.ID)
	if err != nil {
		test.Fatalf("failed to get transaction status from DB: %+v", err)
	}
	if status != expectedTx.Status {
		test.Fatalf("Transaction.Status: %+v (expected) != %+v(actual)", expectedTx.Status, status)
	}

	// Change status and verify that new value is read on next call
	newExpectedStatus := eth.TransactionStatusError
	err = storage.TransactionStorage.UpdateTransactionStatus(expectedTx.ID, newExpectedStatus)
	if err != nil {
		test.Fatalf("failed to update transaction status: %+v", err)
	}

	status, err = storage.TransactionStorage.GetTransactionStatus(expectedTx.ID)
	if err != nil {
		test.Fatalf("failed to get transaction status from DB: %+v", err)
	}
	if status != newExpectedStatus {
		test.Fatalf("Transaction.Status: %+v (expected) != %+v(actual)", newExpectedStatus, status)
	}

	// Check that whole transaction have new status too
	transaction, err := storage.TransactionStorage.GetTransaction(expectedTx.ID)
	if err != nil {
		test.Fatalf("failed to get transaction from DB: %+v", err)
	}
	if transaction.Status != newExpectedStatus {
		test.Fatalf("Transaction.Status: %+v (expected) != %+v(actual)", newExpectedStatus, transaction.Status)
	}
}