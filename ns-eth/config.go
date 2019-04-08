/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package nseth

import (
	"time"

	"github.com/Multy-io/Multy-back/common"
	"github.com/Multy-io/Multy-back/ns-eth/storage"
)

// Configuration is a struct with all service options
type Configuration struct {
	CanaryTest      bool
	Name            string
	GrpcPort        string
	NsqAddress      string
	EthConf         Conf
	ServiceInfo     common.ServiceInfo // why in config?
	NetworkID       int
	ResyncUrl       string
	AbiClientUrl    string
	EtherscanAPIURL string
	EtherscanAPIKey string
	PprofPort       string
	ImmutableBlockDepth uint
	DB              storage.Config
	NSQURL          string
	// Delay between blocks bigger than this would case reconnecting to node.
	MaxBlockDelay   time.Duration
}
