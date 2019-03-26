/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package client

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	// "github.com/Multy-io/Multy-back/eth"
	"github.com/Multy-io/Multy-back/store"

	"github.com/gin-gonic/gin"
	gosocketio "github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
	_ "github.com/jekabolt/slflog"
)

const (
	socketIOOutMsg = "outcoming"
	socketIOInMsg  = "incoming"

	deviceTypeMac     = "mac"
	deviceTypeAndroid = "android"

	topicExchangeDay      = "exchangeDay"
	topicExchangeGdax     = "exchangeGdax"
	topicExchangePoloniex = "exchangePoloniex"
)

const (
	WirelessRoom = "wireless"

	ReceiverOn = "event:receiver:on"
	SenderOn   = "event:sender:on"

	SenderCheck = "event:sender:check"

	StartupReceiverOn         = "event:startup:receiver:on"
	StartupReceiversAvailable = "event:startup:receiver:available"

	Filter = "event:filter"

	// Wireless send

	NewReceiver     = "event:new:receiver"
	SendRaw         = "event:sendraw"
	PaymentSend     = "event:payment:send"
	PaymentReceived = "event:payment:received"

	stopReceive = "receiver:stop"
	stopSend    = "sender:stop"

	msgSend    = "message:send"
	msgRecieve = "message:recieve"
)

func getHeaderDataSocketIO(headers http.Header) (*SocketIOUser, error) {
	userID := headers.Get("userID")
	if len(userID) == 0 {
		return nil, fmt.Errorf("wrong userID header")
	}

	deviceType := headers.Get("deviceType")
	if len(deviceType) == 0 {
		return nil, fmt.Errorf("wrong deviceType header")
	}

	jwtToken := headers.Get("jwtToken")
	if len(jwtToken) == 0 {
		return nil, fmt.Errorf("wrong jwtToken header")
	}
	return &SocketIOUser{
		userID:     userID,
		deviceType: deviceType,
		jwtToken:   jwtToken,
	}, nil
}
func SetSocketIOHandlers(restClient *RestClient, r *gin.RouterGroup, address, nsqAddr string, ratesDB store.UserStore) (*SocketIOConnectedPool, error) {
	// func SetSocketIOHandlers(restClient *RestClient, BTC *btc.BTCConn, ETH *eth.EthController, r *gin.RouterGroup, address, nsqAddr string, ratesDB store.UserStore) (*SocketIOConnectedPool, error) {
	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())
	pool, err := InitConnectedPool(server, address, nsqAddr, ratesDB)
	if err != nil {
		return nil, fmt.Errorf("connection pool initialization: %s", err.Error())
	}
	pool.Server = server

	chart, err := newExchangeChart(ratesDB)
	if err != nil {
		return nil, fmt.Errorf("exchange chart initialization: %s", err.Error())
	}
	pool.chart = chart

	receivers := &sync.Map{} // string UserCode to store.Receiver
	startupReceivers := &sync.Map{}

	senders := []store.Sender{}

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		user, err := getHeaderDataSocketIO(c.RequestHeader())
		if err != nil {
			pool.log.Errorf("get socketio headers: %s", err.Error())
			return
		}
		user.pool = pool
		connectionID := c.Id()
		user.chart = pool.chart

		pool.m.Lock()
		defer pool.m.Unlock()
		userFromPool, ok := pool.users[user.userID]
		if !ok {
			pool.log.Debugf("new user")
			newSocketIOUser(connectionID, user, c, pool.log)
			pool.users[user.userID] = user
			userFromPool = user
		}

		userFromPool.conns[connectionID] = c
		pool.closeChByConnID[connectionID] = userFromPool.closeCh

		sendExchange(user, c)
		pool.log.Debugf("OnConnection done")
	})

	server.On(gosocketio.OnError, func(c *gosocketio.Channel) {
		pool.log.Errorf("Error occurs %s", c.Id())
	})

	//feature logic
	server.On(ReceiverOn, func(c *gosocketio.Channel, data store.Receiver) string {
		pool.log.Debugf("Got message Receiver On:", data)
		receiver := store.Receiver{
			ID:         data.ID,
			UserCode:   data.UserCode,
			CurrencyID: data.CurrencyID,
			NetworkID:  data.NetworkID,
			Amount:     data.Amount,
			Address:    data.Address,
			Socket:     c,
		}

		receivers.Store(receiver.UserCode, receiver)

		return "ok"
	})

	server.On(SenderCheck, func(c *gosocketio.Channel, nearIDs store.NearVisible) []store.Receiver {
		pool.log.Debugf("SenderCheck")
		nearReceivers := []store.Receiver{}

		for _, id := range nearIDs.IDs {
			if res, ok := receivers.Load(id); ok {
				nearReceiver, ok := res.(store.Receiver)
				if ok {
					nearReceivers = append(nearReceivers, nearReceiver)
				}
			}
		}
		c.Emit(SenderCheck, nearReceivers)
		return nearReceivers
	})

	// startup airdrop logic
	server.On(StartupReceiverOn, func(c *gosocketio.Channel, data store.StartupReceiver) string {
		pool.log.Debugf("Got message Startup Receiver On: %v", data)
		receiver := store.StartupReceiver{
			ID:       data.ID,
			UserCode: data.UserCode,
			Socket:   c,
		}
		startupReceivers.Store(receiver.UserCode, receiver)
		return "ok"
	})

	server.On(StartupReceiversAvailable, func(c *gosocketio.Channel, nearIDs store.NearVisible) []store.StartupReceiver {
		pool.log.Debugf("StartupReceiversAvailable event requested")

		nearReceivers := []store.StartupReceiver{}
		userIds := []string{}

		for _, id := range nearIDs.IDs {
			if receiverProto, ok := startupReceivers.Load(id); ok {
				userIds = append(userIds, receiverProto.(store.StartupReceiver).ID)
			}
		}

		if len(userIds) > 0 {
			nearReceivers, err = ratesDB.GetUsersReceiverAddressesByUserIds(userIds)
			if err != nil {
				pool.log.Errorf("An error occurred on GetUsersReceiverAddressesByUserIds: %+v\n", err.Error())
			}
		}

		c.Emit(StartupReceiversAvailable, nearReceivers)
		return nearReceivers
	})

	// on socket disconnection
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		pool.log.Infof("Disconnected %s", c.Id())
		pool.removeUserConn(c.Id())
		receivers.Range(func(userCode, res interface{}) bool {
			receiver, ok := res.(store.Receiver)
			if ok {
				if receiver.Socket.Id() == c.Id() {
					pool.log.Debugf("OnDisconnection:receivers: %v", receiver.Socket.Id())
					receivers.Delete(userCode)
				}
			}
			return true
		})

		startupReceivers.Range(func(userCode, res interface{}) bool {
			receiver, ok := res.(store.StartupReceiver)
			if ok {
				if receiver.Socket.Id() == c.Id() {
					pool.log.Debugf("OnDisconnection:startupReceivers: %v", receiver.Socket.Id())
					startupReceivers.Delete(userCode)
				}
			}
			return true
		})

		for i, sender := range senders {
			if sender.Socket.Id() == c.Id() {
				pool.log.Debugf("OnDisconnection:sender: %v", sender.Socket.Id())
				senders = append(senders[:i], senders[i+1:]...)
				continue
			}
		}
	})

	server.On(stopReceive, func(c *gosocketio.Channel) string {
		pool.log.Debugf("Stop receive %s", c.Id())
		receivers.Range(func(userCode, res interface{}) bool {
			receiver, ok := res.(store.Receiver)
			if ok {
				if receiver.Socket.Id() == c.Id() {
					pool.log.Debugf("stopReceive:receivers: %v", receiver.Socket.Id())
					receivers.Delete(userCode)
				}
			}
			return true
		})

		startupReceivers.Range(func(userCode, res interface{}) bool {
			receiver, ok := res.(store.StartupReceiver)
			if ok {
				if receiver.Socket.Id() == c.Id() {
					pool.log.Debugf("stopReceive:startupReceivers: %v", receiver.Socket.Id())
					startupReceivers.Delete(userCode)
				}
			}
			return true
		})

		return stopReceive + ":ok"
	})

	server.On(stopSend, func(c *gosocketio.Channel) string {
		pool.log.Debugf("Stop send %s", c.Id())
		for i, sender := range senders {
			if sender.Socket.Id() == c.Id() {
				pool.log.Debugf("stopSend:sender: %v", sender.Socket.Id())
				senders = append(senders[:i], senders[i+1:]...)
				continue
			}
		}
		return stopSend + ":ok"

	})

	server.On(msgRecieve, func(c *gosocketio.Channel, msg store.WsMessage) string {
		return ""
	})

	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", server)

	pool.log.Infof("Starting socketIO server on %s address", address)
	go func() {
		pool.log.Panicf("%s", http.ListenAndServe(address, serveMux))
	}()
	return pool, nil
}

// func getMultisig(uStore store.UserStore, msgMultisig *store.MultisigMsg, method int) (*store.Multisig, store.WsMessage, error) {
// 	msg := store.WsMessage{}
// 	multisig, err := uStore.FindMultisig(msgMultisig.UserID, msgMultisig.InviteCode)
// 	if err != nil {
// 		msg = store.WsMessage{
// 			Type:    method,
// 			To:      msgMultisig.UserID,
// 			Date:    time.Now().Unix(),
// 			Payload: "can't join multisig: " + err.Error(),
// 		}
// 	}
// 	return multisig, msg, err
// }

func makeErr(userid, errorStr string, method int) store.WsMessage {
	return store.WsMessage{
		Type:    method,
		To:      userid,
		Date:    time.Now().Unix(),
		Payload: errorStr,
	}
}
