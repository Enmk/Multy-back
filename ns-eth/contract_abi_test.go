package nseth

import (
	"testing"
	
	"github.com/Multy-io/Multy-back/types/eth"

	. "github.com/Multy-io/Multy-back/tests"
	. "github.com/Multy-io/Multy-back/tests/eth"
)

func checkDecodeSmartContractCall(test *testing.T, input string, expected eth.SmartContractMethodInfo) {
	test.Logf("input: %s", input)

	scCall, err := DecodeSmartContractCall(input)
	if err != nil {
		test.Errorf("failed to decode smart contract call: %+v", err)
		return
	}

	if scCall == nil {
		test.Error("decoded smart contract info is null")
		return
	}

	if eq, l, r := TestEqual(expected, *scCall); !eq {
		test.Fatalf("Invalid value: expected != actual\n\texpected: %s\n\tactual  : %s", l, r)
	}
}

func checkDecodeSmartContractCallError(test *testing.T, input string) {
	test.Logf("input: %s", input)

	_, err := DecodeSmartContractCall(input)
	if err == nil {
		test.Errorf("DecodeSmartContractCall expected to fail")
		return
	}
}

func checkDecodeSmartContractEvent(test *testing.T, input string, expected eth.SmartContractEventInfo) {
	test.Logf("input: %s", input)

	scEvent, err := DecodeSmartContractEvent(input)
	if err != nil {
		test.Errorf("failed to decode smart contract event: %+v", err)
		return
	}

	if scEvent == nil {
		test.Error("decoded smart contract info is null")
		return
	}

	if eq, l, r := TestEqual(expected, *scEvent); !eq {
		test.Fatalf("Invalid value: expected != actual\n\texpected: %s\n\tactual  : %s", l, r)
	}
}

func checkDecodeSmartContractEventError(test *testing.T, input string) {
	test.Logf("input: %s", input)

	_, err := DecodeSmartContractEvent(input)
	if err == nil {
		test.Errorf("DecodeSmartContractEvent expected to fail")
		return
	}
}


func TestDecodeSmartContractCall(test *testing.T) {

// https://etherscan.io/tx/0x83daf5344e55af2627b7ae56b842bc70328f82edda40bd93aff8c931adf55d5a
// 	Function: transfer(address to, uint256 value)
// MethodID: 0xa9059cbb
// [0]:  00000000000000000000000079c949c831aadb44d5562f43b38508797c09fa10
// [1]:  0000000000000000000000000000000000000000000000410d586a20a4c00000
	checkDecodeSmartContractCall(test,
		"0xa9059cbb00000000000000000000000079c949c831aadb44d5562f43b38508797c09fa100000000000000000000000000000000000000000000000410d586a20a4c00000",
		eth.SmartContractMethodInfo{
			Name: "transfer(address,uint256)",
			Arguments: []eth.SmartContractMethodArgument{
				eth.HexToAddress("79c949c831aadb44d5562f43b38508797c09fa10"),
				*NewBigIntFromHex("410d586a20a4c00000"),
			},
		})

	checkDecodeSmartContractCallError(test, "")
	checkDecodeSmartContractCallError(test, "0x")
	checkDecodeSmartContractCallError(test, "0xa9059cbb")
	 // trimmed last 2 characters (1byte)
	checkDecodeSmartContractCallError(test, "0xa9059cbb00000000000000000000000079c949c831aadb44d5562f43b38508797c09fa100000000000000000000000000000000000000000000000410d586a20a4c000")
	// unknown signature
	checkDecodeSmartContractCallError(test, "0xa9059cFF00000000000000000000000079c949c831aadb44d5562f43b38508797c09fa100000000000000000000000000000000000000000000000410d586a20a4c00000")
}

func TestDecodeSmartContractEvent(test *testing.T) {
	// https://etherscan.io/tx/0x2b855e6fcc1bcf66031de7e5a407150ca43c510636343524ae0ae80a201e1ce4#eventlog
	// Transfer (index_topic_1 address _from, index_topic_2 address _to, uint256 _value)
	// Topics
	// 0 0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef
	// 1 0x000000000000000000000000676fd83da5960cabc6ec745306793fc3673f76d9
	// 2 0x000000000000000000000000c3cfacce8e454b8b2b058f0be4e4c61e27e765a5
	// Data
	// Hex0000000000000000000000000000000000000000000000004563918244f40000

	checkDecodeSmartContractEvent(test,
		"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000676fd83da5960cabc6ec745306793fc3673f76d9000000000000000000000000c3cfacce8e454b8b2b058f0be4e4c61e27e765a50000000000000000000000000000000000000000000000004563918244f40000",
		eth.SmartContractEventInfo{
			Name: "Transfer(address,address,uint256)",
			Arguments: []eth.SmartContractEventArgument{
				eth.HexToAddress("676fd83da5960cabc6ec745306793fc3673f76d9"),
				eth.HexToAddress("c3cfacce8e454b8b2b058f0be4e4c61e27e765a5"),
				*NewBigIntFromHex("4563918244f40000"),
			},
		})

	checkDecodeSmartContractEventError(test, "")
	checkDecodeSmartContractEventError(test, "0x")
	checkDecodeSmartContractEventError(test, "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")
	// trimmed few last bytes
	checkDecodeSmartContractEventError(test, "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef000000000000000000000000676fd83da5960cabc6ec745306793fc3673f76d9000000000000000000000000c3cfacce8e454b8b2b058f0be4e4c61e27e765a50000000000000000000000000000000000000000000000004563918244f4")
	// unknown signature
	checkDecodeSmartContractEventError(test, "0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3FF000000000000000000000000676fd83da5960cabc6ec745306793fc3673f76d9000000000000000000000000c3cfacce8e454b8b2b058f0be4e4c61e27e765a50000000000000000000000000000000000000000000000004563918244f40000")
}