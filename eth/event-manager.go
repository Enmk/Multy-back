package eth

import (
	"encoding/json"

	"github.com/jekabolt/slf"
	"github.com/pkg/errors"

	nsq "github.com/bitly/go-nsq"

	common "github.com/Multy-io/Multy-back/common/eth"
)

type TransactionStatusHandler interface {
	HandleTransactionStatus(tx common.TransactionStatusEvent) error
}

type BlockHandler interface {
	HandleBlock(block common.BlockHeader) error
}

// TODO: rename to EventManager
type EventManager struct {
	addressNSQ          string
	txStatusHandler     TransactionStatusHandler
	blockHandler        BlockHandler
	nsqConsumerTxStatus *nsq.Consumer
	nsqConsumerBlock    *nsq.Consumer
	nsqProducer         *nsq.Producer
	log                 slf.StructuredLogger
}

func NewEventHandler(nsqAddr string, blockHandler BlockHandler, txStatusHandler TransactionStatusHandler) (*EventManager, error) {
	if txStatusHandler == nil {
		return nil, errors.New("Not set TxStatus Handler")
	}
	if blockHandler == nil {
		return nil, errors.New("not set blockHandler")
	}

	eventHandler := EventManager{
		log:             slf.WithContext("eth NSQ").WithCaller(slf.CallerShort),
		addressNSQ:      nsqAddr,
		txStatusHandler: txStatusHandler,
		blockHandler:    blockHandler,
	}

	var err error
	config := nsq.NewConfig()
	// Set handler for Transaction status
	eventHandler.nsqConsumerTxStatus, err = nsq.NewConsumer(common.NSQETHTxStatus, "tx", config)
	if err != nil {
		return nil, errors.Wrapf(err, "new nsq consumer new tx status: ")
	}

	eventHandler.nsqConsumerTxStatus.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var txStatus common.TransactionStatusEvent
		err := json.Unmarshal(msgRaw, &txStatus)
		if err != nil {
			return errors.Wrap(err, "Wrong unmarshal message from NSQ")
		}
		err = eventHandler.txStatusHandler.HandleTransactionStatus(txStatus)
		if err != nil {
			return errors.Wrapf(err, "Wrong processing data %v", txStatus)
		}
		return nil
	}))
	// TODO: if we will go from NSQD to NSQLookupd then here and another place
	// we will change ConnectToNSQD to ConnectToNSQLookupd
	err = eventHandler.nsqConsumerTxStatus.ConnectToNSQD(eventHandler.addressNSQ)
	if err != nil {
		return nil, errors.Wrap(err, "Error on consumer connect to nsq err")
	}

	// Set handler for block hendler
	eventHandler.nsqConsumerBlock, err = nsq.NewConsumer(common.NSQETHSendRawTransaction, "block", config)
	if err != nil {
		return nil, errors.Wrap(err, "new nsq consumer block")
	}

	eventHandler.nsqConsumerBlock.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var block common.BlockHeader
		err := json.Unmarshal(msgRaw, &block)
		if err != nil {
			return errors.Wrap(err, "Wrong unmarshal message from NSQ")
		}
		err = eventHandler.blockHandler.HandleBlock(block)
		if err != nil {
			return errors.Wrapf(err, "Wrong processing data %v", block)
		}
		return nil
	}))
	err = eventHandler.nsqConsumerBlock.ConnectToNSQD(eventHandler.addressNSQ)
	if err != nil {
		return nil, errors.Wrap(err, "error on consumer connect to nsq err")
	}

	// Create NSQ producer
	eventHandler.nsqProducer, err = nsq.NewProducer(eventHandler.addressNSQ, config)
	if err != nil {
		return nil, errors.Wrapf(err, "error on Producer creator on address: %s", eventHandler.addressNSQ)
	}

	return &eventHandler, nil
}

func (self *EventManager) EmitNewAddressEvent(address common.Address) error {
	return self.emitEvent(common.NSQETHNewAddress, address)
}

func (self *EventManager) EmitRawTransactionEvent(rawTx common.RawTransaction) error {
	return self.emitEvent(common.NSQETHSendRawTransaction, rawTx)
}

func (self *EventManager) emitEvent(topic string, data interface{}) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return errors.Wrapf(err, " error on Marshal event: %v", data)
	}
	err = self.nsqProducer.Publish(topic, raw)
	if err != nil {
		return errors.Wrapf(err, " failed to publish event: %s data: [%v]", topic, raw)
	}
	return nil
}

func (self *EventManager) Close() {
	self.nsqProducer.Stop()
	self.nsqConsumerBlock.Stop()
	self.nsqConsumerTxStatus.Stop()
}