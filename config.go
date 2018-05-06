/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package multyback

import (
	"github.com/Appscrunch/Multy-back-exchange-service/core"
	"github.com/Appscrunch/Multy-back/client"
	"github.com/Appscrunch/Multy-back/store"
)

// Configuration is a struct with all service options
type Configuration struct {
	Name                   string
	Database               store.Conf
	SocketioAddr           string
	RestAddress            string
	Firebase               client.FirebaseConf
	NSQAddress             string
	BTCNodeAddress         string
	DonationAddresses      []store.DonationInfo
	MultyVerison           store.ServerConfig
	ExchangerConfiguration core.ManagerConfiguration
	ServicesInfo           []store.ServiceInfo

	SupportedNodes []store.CoinType
}
