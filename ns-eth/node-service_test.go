package nseth

import (
	"testing"

	"flag"
	"os"
	l "log"
	"time"

	"github.com/jekabolt/config"
	"github.com/onrik/ethrpc"
	"github.com/Multy-io/Multy-back/types/eth"
)

var (
	conf Configuration
)
func init() {
    flag.String("ConfigPath", "", "path to config file to allow providing node-service configs")
}

type mockAddressLookup struct {}
func (*mockAddressLookup) IsAddressExists(eth.Address) bool {
	return false
}

func newNodeClient() *NodeClient {

	client := NewClient(&conf.EthConf, &mockAddressLookup{})
	select {
	case <- client.ready:
		break;
	case <- time.After(1 * time.Second):
		l.Fatalf("NewClient timed out")
	}

	return client
}

func TestMain(m *testing.M) {
	// You may want to set --ConfigPath=absolute-path-to-config-file
	config.ReadGlobalConfig(&conf, "NS-ETH config")

	os.Exit(m.Run())
}

func TestProcessTransaction(test *testing.T) {
	client := newNodeClient()
	defer client.Shutdown()

	// Kryptokitties auction bid which results in transfer.
	transaction := ethrpc.Transaction{
		Hash: "0x975914f6a8b7e62324ec22a8ebe478ae7480725e8886f0fb7c0539acae26512f",
		Input: "0x0",
	}
	client.parseETHTransaction(transaction, 7371365, false)
}