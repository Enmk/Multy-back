package nsethprotobuf

import (
	"testing"

	"math/big"

	"github.com/Multy-io/Multy-back/types/eth"

	. "github.com/Multy-io/Multy-back/tests"
	. "github.com/Multy-io/Multy-back/tests/eth"
)

func toAddressPtr(address string ) *eth.Address {
	addr := ToAddress(address)
	return &addr
}

func checkTransactionToAndFromProtobuf(test *testing.T, transaction eth.Transaction) {
	pbTx, err := TransactionToProtobuf(transaction)
	if err != nil {
		test.Errorf("Failed to convert transaction to protobuf: %+v", err)
		return
	}

	transcodedTx, err := TransactionFromProtobuf(*pbTx)
	if err != nil {
		test.Errorf("Failed to convert transaction from protobuf: %+v", err)
		return
	}

	if equal, l, r := TestEqual(transaction, *transcodedTx); !equal {
		test.Errorf("Invalid value: expected != actual\nexpected:\n%s\nactual:\n%s", l, r)
		return
	}
}

func checkTransactionToAndFromProtobufNoCheck(test *testing.T, transaction eth.Transaction) {
	pbTx, err := TransactionToProtobuf(transaction)
	if err != nil {
		test.Errorf("Failed to convert transaction to protobuf: %+v", err)
		return
	}

	transcodedTx, err := TransactionFromProtobuf(*pbTx)
	if err != nil {
		test.Errorf("Failed to convert transaction from protobuf: %+v", err)
		return
	}

	if transcodedTx == nil {
		test.Errorf("Failed to convert transaction from protobuf: got nil result")
	}
}

func checkTransactionToProtobufError(test *testing.T, transaction eth.Transaction) {
	pbTx, err := TransactionToProtobuf(transaction)
	if err != nil {
		return
	}

	transcodedTx, err := TransactionFromProtobuf(*pbTx)
	if err == nil {
		test.Errorf("Expected to fail, got: %+v", transcodedTx)
		return
	}
}

func sampleTransaction() eth.Transaction {
	// NOTE: setting pointer-arguments works too
	// but converting from protobuf to eth Transaction produces value-arguments.
	// so comparison would fail.
	return eth.Transaction{
		ID:       ToTxHash("mock transaction hash"),
		Sender:   ToAddress("mock sender address"),
		Receiver: ToAddress("mock receiver address"),
		Payload:  []byte("mock payload"),
		Amount:   eth.Amount{*big.NewInt(1337)},
		Nonce:    42,
		Fee: eth.TransactionFee{
			GasLimit: 21*1000,
			GasPrice: 42*eth.GWei,
		},
		CallInfo: &eth.SmartContractCallInfo{
			Status: eth.SmartContractCallStatusOk,
			DeployedAddress: toAddressPtr("mock contract deployed address"),
			Method: &eth.SmartContractMethodInfo{
				Name: "mock method",
				Arguments: []eth.SmartContractMethodArgument{
					ToAddress("method argument address"),
					*big.NewInt(123),
				},
			},
			Events: []eth.SmartContractEventInfo{
				{
					Name: "mock event1",
					Arguments: []eth.SmartContractEventArgument{
						ToAddress("event argument address"),
					},
				},
				{
					Name: "mock event2",
					Arguments: []eth.SmartContractEventArgument{
							ToAddress("event argument address"),
							"string value argument 1",
							*big.NewInt(1016),
							ToTxHash("mock tx hash"),
							false,
							true,
					},
				},
			},
		},
	}
}

func TestTransactionToProtobufAndBack(test *testing.T) {
	tx1 := sampleTransaction()
	tx1.CallInfo.DeployedAddress = nil
	checkTransactionToAndFromProtobuf(test, tx1)

	// Check that pointer arguments work too, however, since we do pointer to values on convertion, do not compare for equality.
	tx2 := tx1
	args := tx2.CallInfo.Events[1].Arguments
	args = append(args, big.NewInt(123456), toAddressPtr("*Address"), &eth.Hash{})
	tx2.CallInfo.Events[1].Arguments = args

	checkTransactionToAndFromProtobufNoCheck(test, tx2)
}

func TestTransactionToProtobufAndBackError(test *testing.T){
	errTx1 := sampleTransaction()
	errTx1.CallInfo.Method.Arguments[0] = struct{}{}
	checkTransactionToProtobufError(test, errTx1)

	errTx2 := sampleTransaction()
	errTx2.CallInfo.Method.Arguments[0] = nil
	checkTransactionToProtobufError(test, errTx2)
}