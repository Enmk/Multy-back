package server

import (
	"encoding/json"

	"github.com/jekabolt/slf"
	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/types/eth"
	nsq "github.com/bitly/go-nsq"
)

type AddressHandler interface {
	HandleNewAddress(address eth.Address) error
}

type RawTxHandler interface {
	HandleSendRawTx(rawTx eth.RawTransaction) error
}

type EventManager struct {
	addressNSQ            string
	addressHandler        AddressHandler
	rawTxHandler          RawTxHandler
	nsqProducer           *nsq.Producer
	nsqConsumerNewAddress *nsq.Consumer
	nsqConsumerRawTx      *nsq.Consumer
	log                   slf.StructuredLogger
}

func NewEventManager(nsqAddr string, addressHandler AddressHandler, rawTxHandler RawTxHandler) (*EventManager, error) {
	if addressHandler == nil {
		return nil, errors.New("Not set addressHandler")
	}
	if rawTxHandler == nil {
		return nil, errors.New("Not set rawTxHandler")
	}
	eventHandler := EventManager{
		log:            slf.WithContext("ns-eth NSQ").WithCaller(slf.CallerShort),
		addressNSQ:     nsqAddr,
		addressHandler: addressHandler,
		rawTxHandler:   rawTxHandler,
	}

	var err error
	config := nsq.NewConfig()
	// Set handler for new address
	eventHandler.nsqConsumerNewAddress, err = nsq.NewConsumer(eth.NSQETHNewAddress, "address", config)
	if err != nil {
		return nil, errors.Wrapf(err, "new nsq consumer new address: ")
	}

	eventHandler.nsqConsumerNewAddress.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var address eth.Address
		err := json.Unmarshal(msgRaw, &address)
		if err != nil {
			return errors.Wrap(err, "Wrong unmarshal message from NSQ")
		}
		err = eventHandler.addressHandler.HandleNewAddress(address)
		if err != nil {
			return errors.Wrapf(err, "Wrong processing data %v", address)
		}
		return nil
	}))
	err = eventHandler.nsqConsumerNewAddress.ConnectToNSQD(nsqAddr)
	if err != nil {
		return nil, errors.Wrapf(err, "Error on consumer connect to nsq err: ")
	}

	// Set handler for rawTransaction
	eventHandler.nsqConsumerRawTx, err = nsq.NewConsumer(eth.NSQETHSendRawTransaction, "tx", config)
	if err != nil {
		return nil, errors.Wrapf(err, "new nsq consumer raw tx: ")
	}

	eventHandler.nsqConsumerRawTx.AddHandler(nsq.HandlerFunc(func(message *nsq.Message) error {
		msgRaw := message.Body
		var rawTx eth.RawTransaction
		err := json.Unmarshal(msgRaw, &rawTx)
		if err != nil {
			return errors.Wrap(err, "Wrong unmarshal message from NSQ")
		}
		err = eventHandler.rawTxHandler.HandleSendRawTx(rawTx)
		if err != nil {
			return errors.Wrapf(err, "Wrong processing data %v", rawTx)
		}
		return nil
	}))
	err = eventHandler.nsqConsumerRawTx.ConnectToNSQD(eventHandler.addressNSQ)
	if err != nil {
		return nil, errors.Wrapf(err, "error on consumer connect to nsq err: ")
	}

	// Create NSQ producer
	eventHandler.nsqProducer, err = nsq.NewProducer(nsqAddr, config)
	if err != nil {
		return nil, errors.Wrapf(err, "error on Producer creator on address: %s", eventHandler.addressNSQ)
	}

	return &eventHandler, nil
}

func (self *EventManager) EmitTransactionStatusEvent(tx eth.TransactionWithStatus) error {
	return self.emitEvent(eth.NSQETHTxStatus, tx)
}

func (self *EventManager) EmitNewBlock(block eth.BlockHeader) error {
	return self.emitEvent(eth.NSQETHNewBlock, block)
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
	self.nsqConsumerNewAddress.Stop()
	self.nsqConsumerRawTx.Stop()
}
