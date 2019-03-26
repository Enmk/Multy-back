package nseth

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	nsq "github.com/bitly/go-nsq"

	"github.com/Multy-io/Multy-back/common/eth"

	. "github.com/Multy-io/Multy-back/tests"
	. "github.com/Multy-io/Multy-back/tests/eth"
)

var (
	addressNSQ string = GetenvOrDefault("NSQ_ENDPOINT", "127.0.0.1:4150")
	nsqConfig *nsq.Config = nsq.NewConfig()
)

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

func TestRegisterEvents(test *testing.T) {
	testAddressHandler := testAddressHandler{}
	testRawTxHandler := testRawTxHandler{}
	eventManager, err := NewEventManager(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()
	if err != nil {
		test.Errorf("NewEventManager return error: %v", err)
	}

	testNsqRegisterAdddress, err := nsq.NewProducer(addressNSQ, nsqConfig)
	defer testNsqRegisterAdddress.Stop()
	if err != nil {
		test.Error("Error create producer")
	}

	var testAddress eth.Address = ToAddress("test address")
	addressJSON, err := json.Marshal(testAddress)
	if err != nil {
		test.Error("Error marshal string to addressJSON(byte[])")
	}

	err = testNsqRegisterAdddress.Publish(eth.NSQETHNewAddress, addressJSON)
	if err != nil {
		test.Error("Error send addressJSON(byte[]) to NSQ")
	}

	testNsqRegisterRawTransacion, err := nsq.NewProducer(addressNSQ, nsqConfig)
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
	eventManager, err := NewEventManager(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()

	expectedStatusEvent := eth.TransactionStatusEvent{
		ID:        ToTxHash("test-ID"),
		Status:    eth.TransactionStatusInMempool,
		BlockHash: ToBlockHash("Block hash"),
	}

	// Register handler for check get message
	testNsqConsumerTxStatus, err := nsq.NewConsumer(eth.NSQETHTxStatus, "tx", nsqConfig)
	if err != nil {
		test.Errorf("new nsq consumer tx status test : " + err.Error())
	}

	testNsqConsumerTxStatus.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var actualStatusEvent eth.TransactionStatusEvent
		err := json.Unmarshal(msgRaw, &actualStatusEvent)
		if err != nil {
			test.Errorf("bad status after unmarshal with error: %+v,   %v", err, msgRaw)
			return err
		}
		if equal, l, r := TestEqual(expectedStatusEvent, actualStatusEvent); !equal {
			test.Errorf("event expected != actual\nexpected:\n%s\actual:\n%s", l, r)
		}
		return nil
	}))
	err = testNsqConsumerTxStatus.ConnectToNSQD(addressNSQ)
	if err != nil {
		test.Errorf("error on consumer connect to nsq err: %v", err)
	}

	// Send message to nsq
	err = eventManager.EmitTransactionStatusEvent(expectedStatusEvent)
	if err != nil {
		test.Errorf("Failed to emit an event : %+v", err)
	}

	time.Sleep(20 * time.Millisecond)
}

func TestBlockHandler(test *testing.T) {
	testAddressHandler := testAddressHandler{}
	testRawTxHandler := testRawTxHandler{}
	eventManager, err := NewEventManager(addressNSQ, &testAddressHandler, &testRawTxHandler)
	defer eventManager.Close()

	var testId eth.BlockHash = ToBlockHash("zxc")
	testBlockHeader := eth.BlockHeader{
		ID:     testId,
		Height: 1,
		Parent: testId,
	}

	// Register handler for check get message
	testNsqConsumerBlockHeader, err := nsq.NewConsumer(eth.NSQETHNewBlock, "block", nsqConfig)
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
