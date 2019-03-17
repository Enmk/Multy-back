package server

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/Multy-io/Multy-back/types/eth"
	nsq "github.com/bitly/go-nsq"
)

var addressNSQ string
var config *nsq.Config

type testAddressHandler struct {
	addresses []eth.Address
}

func (self *testAddressHandler) HandleNewAddress(address eth.Address) error {
	self.addresses = append(self.addresses, address)
	return nil
}

type testRawTxHandler struct {
	rawTxs []eth.RawTransaction
}

func (self *testRawTxHandler) HandleSendRawTx(rawTx eth.RawTransaction) error {
	self.rawTxs = append(self.rawTxs, rawTx)
	return nil
}

func TestMain(m *testing.M) {
	if os.Getenv("NSQ_ENDPOINT") != "" {
		addressNSQ = os.Getenv("NSQ_ENDPOINT")
	} else {
		addressNSQ = "127.0.0.1:4150"
	}
	config = nsq.NewConfig()
	os.Exit(m.Run())
}

func TestRegisterEvents(test *testing.T) {
	testAddressHandler := testAddressHandler{}
	testRawTxHandler := testRawTxHandler{}
	eventManager, err := NewEventHandler(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()
	if err != nil {
		test.Errorf("NewEventHandler return error: %v", err)
	}

	testNsqRegisterAdddress, err := nsq.NewProducer(addressNSQ, config)
	defer testNsqRegisterAdddress.Stop()
	if err != nil {
		test.Error("Error create producer")
	}

	var testAddress eth.Address = "test address"
	addressJSON, err := json.Marshal(testAddress)
	if err != nil {
		test.Error("Error marshal string to addressJSON(byte[])")
	}

	err = testNsqRegisterAdddress.Publish(eth.NSQETHNewAddress, addressJSON)
	if err != nil {
		test.Error("Error send addressJSON(byte[]) to NSQ")
	}

	testNsqRegisterRawTransacion, err := nsq.NewProducer(addressNSQ, config)
	defer testNsqRegisterRawTransacion.Stop()
	if err != nil {
		test.Error("Error create producer")
	}

	var testRawTx eth.RawTransaction = "test Tx"
	txJSON, err := json.Marshal(testRawTx)
	if err != nil {
		test.Error("Error marshal string to txJSON(byte[])")
	}

	err = testNsqRegisterRawTransacion.Publish(eth.NSQETHSendRawTransaction, txJSON)
	if err != nil {
		test.Error("Error send txJSON(byte[]) to NSQ")
	}

	time.Sleep(10 * time.Millisecond)

	if len(testAddressHandler.addresses) != 1 {
		test.Fatal("Wrong number message in nsq")
	}
	if testAddressHandler.addresses[0] != testAddress {
		test.Error("Wrong value geting from nsq")
	}
	if len(testRawTxHandler.rawTxs) != 1 {
		test.Fatal("Wrong number message in nsq")
	}
	if testRawTxHandler.rawTxs[0] != testRawTx {
		test.Error("Wrong value geting from nsq")
	}
}

func TestTxStatusHandler(test *testing.T) {
	testAddressHandler := testAddressHandler{}
	testRawTxHandler := testRawTxHandler{}
	eventManager, err := NewEventHandler(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()

	txWithStatusMempool := eth.TransactionWithStatus{
		Transaction: eth.Transaction{
			ID: "test-ID",
		},
		Status: eth.TransactionStatusInMempool,
	}

	// Register handler for check get message
	testNsqConsumerTxStatus, err := nsq.NewConsumer(eth.NSQETHTxStatus, "tx", config)
	if err != nil {
		test.Errorf("new nsq consumer tx status test : " + err.Error())
	}

	testNsqConsumerTxStatus.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var txWithStatus eth.TransactionWithStatus
		err := json.Unmarshal(msgRaw, &txWithStatus)
		if err != nil {
			test.Errorf("bad status after unmarshal with error: %v,   %v", err, msgRaw)
			return err
		}
		if !reflect.DeepEqual(txWithStatusMempool, txWithStatus) {
			test.Error("input wrong object that actual")
		}
		return nil
	}))
	err = testNsqConsumerTxStatus.ConnectToNSQD(addressNSQ)
	if err != nil {
		test.Errorf("error on consumer connect to nsq err: %v", err)
	}
	// Send message to nsq
	eventManager.EmitTransactionStatusEvent(txWithStatusMempool)
	time.Sleep(20 * time.Millisecond)
}

func TestBlockHandler(test *testing.T) {
	testAddressHandler := testAddressHandler{}
	testRawTxHandler := testRawTxHandler{}
	eventManager, err := NewEventHandler(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()

	var testId eth.BlockHash = "zxc"
	testBlockHeader := eth.BlockHeader{
		ID:     testId,
		Height: 1,
		Parent: testId,
	}

	// Register handler for check get message
	testNsqConsumerBlockHeader, err := nsq.NewConsumer(eth.NSQETHNewBlock, "block", config)
	if err != nil {
		test.Errorf("new nsq consumer block test : " + err.Error())
	}

	testNsqConsumerBlockHeader.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var block eth.BlockHeader
		err := json.Unmarshal(msgRaw, &block)
		if err != nil {
			test.Errorf("bad status after unmarshal with error: %v", err)
			return err
		}
		if !reflect.DeepEqual(testBlockHeader, block) {
			test.Error("input wrong object that actual")
			return errors.New("input wrong object that actual")
		}
		return nil
	}))
	err = testNsqConsumerBlockHeader.ConnectToNSQD(addressNSQ)
	if err != nil {
		test.Errorf("error on consumer connect to nsq err: %v", err)
	}
	// Send message to nsq
	eventManager.EmitNewBlock(testBlockHeader)
	time.Sleep(10 * time.Millisecond)
}
