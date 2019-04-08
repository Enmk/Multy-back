package nseth

import (
	"testing"

	"flag"
	"os"
	l "log"

	"github.com/jekabolt/config"
	// "github.com/onrik/ethrpc"
	"github.com/Multy-io/Multy-back/common/eth"
)

var (
	conf Configuration
)
func init() {
    flag.String("ConfigPath", "", "path to config file to allow providing node-service configs")
}

type mockAddressLookup struct {}
func (*mockAddressLookup) IsKnownAddress(eth.Address) bool {
	return false
}

type mockHandler struct{}
func (*mockHandler) HandleBlock(eth.BlockHeader) {}
func (*mockHandler) HandleTransaction(eth.Transaction) {}
func (*mockHandler) RequestReconnect(error) {}

func newNodeClient() *NodeClient {

	mockHandler := mockHandler{}
	client, err := NewClient(&conf.EthConf, &mockAddressLookup{}, &mockHandler, &mockHandler, &mockHandler)
	if err != nil {
		l.Fatalf("failed to create a NodeClient: %+v", err)
	}

	return client
}

func TestMain(m *testing.M) {
	// You may want to set --ConfigPath=absolute-path-to-config-file
	config.ReadGlobalConfig(&conf, "NS-ETH config for tests")

	os.Exit(m.Run())
}

// func TestProcessTransaction(test *testing.T) {
// 	client := newNodeClient()
// 	defer client.Shutdown()

// 	// Kryptokitties auction bid which results in transfer.
// 	transaction := ethrpc.Transaction{
// 		Hash: "0x975914f6a8b7e62324ec22a8ebe478ae7480725e8886f0fb7c0539acae26512f",
// 		Input: "0x0",
// 	}
// 	client.HandleEthTransaction(transaction, 7371365, false)
// }

// func TestDecodeTransactionCallInfo(test *testing.T) {
// 	receipt := &ethrpc.TransactionReceipt{
// 		TransactionHash: "0x3af2d5ae761d7ac43a494ec5b3adf5e18878f540749fd7fe8e498599881a2748",
// 		TransactionIndex: 0,
// 		BlockHash: "0x83299a722b9161205db3d1387302e94df21e36f72a3be15346052fea3531f923",
// 		BlockNumber: 7444344,
// 		CumulativeGasUsed: 52587,
// 		GasUsed: 52587,
// 		ContractAddress: "",
// 		Logs: []ethrpc.Log{
// 			{
// 				Removed: false,
// 				LogIndex: 0,
// 				TransactionIndex: 0,
// 				TransactionHash: "0x3af2d5ae761d7ac43a494ec5b3adf5e18878f540749fd7fe8e498599881a2748",
// 				BlockNumber: 7444344,
// 				BlockHash: "0x83299a722b9161205db3d1387302e94df21e36f72a3be15346052fea3531f923",
// 				Address: "0x9f235d23354857efe6c541db92a9ef1877689bcb",
// 				Data: "0x000000000000000000000000000000000000000000000cb49b44ba602d800000",
// 				Topics: []string{
// 					"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
// 					"0x000000000000000000000000f73337e1623fd703d48de13daf80a01da4e132fb",
// 					"0x00000000000000000000000054880e83464741aa2f2234c093e33454c810b0ed",
// 				},
// 			},
// 		},
// 		LogsBloom: "0x00000000000000000000000020000000000000000000000000000000000000400000000000000000100000000000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000080000000000000000000400000000000000000000000000000000100010000000080000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000008000000000002000000000000000000000000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000",
// 		Root: "",
// 		Status: 1,
// 	}

// 	txIndex := int(0)
// 	tx := ethrpc.Transaction{
// 		Hash: "0x3af2d5ae761d7ac43a494ec5b3adf5e18878f540749fd7fe8e498599881a2748",
// 		Nonce: 8,
// 		BlockHash: "0x83299a722b9161205db3d1387302e94df21e36f72a3be15346052fea3531f923",
// 		BlockNumber: 824635666904,
// 		TransactionIndex: &txIndex,
// 		From: "0xf73337e1623fd703d48de13daf80a01da4e132fb",
// 		To: "0x9f235d23354857efe6c541db92a9ef1877689bcb",
// 		Value: *big.NewInt(0),
// 		Gas: 60000,
// 		GasPrice: *big.NewInt(100000000000),
// 		Input: "0xa9059cbb00000000000000000000000054880e83464741aa2f2234c093e33454c810b0ed000000000000000000000000000000000000000000000cb49b44ba602d800000",
// 	}

// 	callInfo, err := decodeTransactionCallInfo(tx, receipt)
// 	test.Logf("parsed tx and receipt into %#v, %+v", callInfo, err)
// }