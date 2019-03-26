/*
Copyright 2018 Idealnaya rabota LLC
Licensed under Multy.io license.
See LICENSE for details
*/
package main

import (
	"flag"
	"fmt"

	"net/http"
	_ "net/http/pprof"

	ns "github.com/Multy-io/Multy-back/ns-eth"
	"github.com/Multy-io/Multy-back/store"
	"github.com/jekabolt/config"
	"github.com/jekabolt/slf"
	_ "github.com/jekabolt/slflog"
)

var (
	log = slf.WithContext("ns-eth").WithCaller(slf.CallerShort)

	// Set externaly during build
	branch    string
	commit    string
	lasttag   string
	buildtime string

	memprofile = flag.String("memprofile", "", "write memory profile to `file`")
)

var globalOpt = ns.Configuration{
	CanaryTest:          false,
	Name:                "eth-node-service",
	ImmutableBlockDepth: 50, // with block once 15 seconds, 50 blocks is approx 12.5 minutes
}

func main() {
	log.Info("============================================================")
	log.Info("Node service ETH starting")
	log.Infof("branch: %s", branch)
	log.Infof("commit: %s", commit)
	log.Infof("build time: %s", buildtime)

	log.Info("Reading configuration...")
	config.ReadGlobalConfig(&globalOpt, "multy configuration")
	log.Infof("CONFIGURATION=%+v", globalOpt)

	if globalOpt.CanaryTest == true {
		log.Info("This is a CanaryTest run, quitting immediatelly...")
		return
	}

	globalOpt.ServiceInfo = store.ServiceInfo{
		Branch:    branch,
		Commit:    commit,
		Buildtime: buildtime,
	}

	service := ns.NodeService{}
	node, err := service.Init(&globalOpt)
	if err != nil {
		log.Fatalf("Server initialization failed:\n%+v", err)
	}
	fmt.Println(node)

	http.ListenAndServe(globalOpt.PprofPort, nil)

	block := make(chan bool)
	<-block
}
