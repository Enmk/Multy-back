/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package main

import (
	"fmt"

	"github.com/KristinaEtc/config"
	_ "github.com/KristinaEtc/slflog"

	multy "github.com/Appscrunch/Multy-back"
	"github.com/Appscrunch/Multy-back-exchange-service/exchange-rates"
	"github.com/Appscrunch/Multy-back/store"
	"github.com/KristinaEtc/slf"
)

var (
	log = slf.WithContext("main")

	branch    string
	commit    string
	buildtime string
	lasttag   string
	// TODO: add all default params
	globalOpt = multy.Configuration{
		Name: "my-test-back",
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
		// Etherium: ethereum.Conf{
		// 	Address: "88.198.47.112",
		// 	RpcPort: ":18545",
		// 	WsPort:  ":8545",
		// },
	}
)

func main() {
	config.ReadGlobalConfig(&globalOpt, "multy configuration")

	log.Error("--------------------------------new multy back server session")
	log.Infof("CONFIGURATION=%+v", globalOpt)

	log.Infof("branch: %s", branch)
	log.Infof("commit: %s", commit)
	log.Infof("build time: %s", buildtime)
	log.Infof("tag: %s", lasttag)

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

	ch := make(chan []*exchangeRates.Exchange)
	go mu.Rates.Exchanger.Subscribe(ch, 1, []string{"BTC", "ETH"}, "USDT")

	go func() {
		for ex := range ch {
			for _, tic := range ex {
				for name, e := range tic.Tickers {
					fmt.Printf("ticker = %v name = %v \n", e, name)
				}
			}
		}
	}()

	if err = mu.Run(); err != nil {
		log.Fatalf("Server running: %s\n", err.Error())
	}
}
