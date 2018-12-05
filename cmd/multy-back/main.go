/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package main

import (
	"github.com/jekabolt/config"
	_ "github.com/jekabolt/slflog"

	multy "github.com/Multy-io/Multy-back"
	"github.com/Multy-io/Multy-back/store"
	"github.com/jekabolt/slf"
	_ "github.com/swaggo/gin-swagger"              // gin-swagger middleware
	_ "github.com/swaggo/gin-swagger/swaggerFiles" // swagger embed files
)

var (
	log = slf.WithContext("multy-back")

	// Set externaly during build
	branch    string
	commit    string
	lasttag   string
	buildtime string

	// TODO: add all default params
	globalOpt = multy.Configuration{
		CanaryTest: false,
		Name:       "my-test-back",
		Database: store.Conf{
			Address:             "localhost:27017",
			DBUsers:             "userDB-test",
			DBFeeRates:          "BTCMempool-test",
			DBTx:                "DBTx-test",
			DBStockExchangeRate: "dev-DBStockExchangeRate",
			Username:            "Username",
			Password:            "Password",
		},
		RestAddress:    "localhost:7778",
		SocketioAddr:   "localhost:7780",
		NSQAddress:     "nsq:4150",
		BTCNodeAddress: "localhost:18334",
	}
)

func main() {
	log.Info("============================================================")
	log.Infof("branch: %s", branch)
	log.Infof("commit: %s", commit)
	log.Infof("build time: %s", buildtime)
	log.Infof("tag: %s", lasttag)

	config.ReadGlobalConfig(&globalOpt, "multy configuration")
	log.Infof("CONFIGURATION=%+v", globalOpt.SupportedNodes)

	if globalOpt.CanaryTest == true {
		log.Info("This is a CanaryTest run, quitting immediatelly...")
		return
	}

	sc := store.ServerConfig{
		BranchName: branch,
		CommitHash: commit,
		Build:      buildtime,
		Tag:        lasttag,
	}

	globalOpt.MultyVerison = sc

	mu, err := multy.Init(&globalOpt)
	if err != nil {
		log.Fatalf("Server initialization: %s\n", err.Error())
	}

	if err = mu.Run(); err != nil {
		log.Fatalf("Server running: %s\n", err.Error())
	}

}
