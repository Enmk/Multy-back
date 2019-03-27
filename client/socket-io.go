/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package client

import (
	"fmt"
	"net/http"
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

	Filter = "event:filter"

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

	// on socket disconnection
	server.On(gosocketio.OnDisconnection, func(c *gosocketio.Channel) {
		pool.log.Infof("Disconnected %s", c.Id())
		pool.removeUserConn(c.Id())
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
