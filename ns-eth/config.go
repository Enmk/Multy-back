/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"github.com/Multy-io/Multy-back/common"
	"github.com/Multy-io/Multy-back/ns-eth/storage"
)

// Configuration is a struct with all service options
type Configuration struct {
	CanaryTest      bool
	Name            string
	GrpcPort        string
	EthConf         Conf
	ServiceInfo     common.ServiceInfo
	NetworkID       int
	ResyncUrl       string
	AbiClientUrl    string
	EtherscanAPIURL string
	EtherscanAPIKey string
	PprofPort       string
	ImmutableBlockDepth uint
	DB              storage.Config
	NSQURL          string
}
