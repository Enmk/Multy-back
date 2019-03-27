package nsethprotobuf

import (
	"testing"

	"math/big"

	"github.com/Multy-io/Multy-back/common/eth"

	. "github.com/Multy-io/Multy-back/tests"
	. "github.com/Multy-io/Multy-back/tests/eth"
)

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

func TestTransactionToProtobufAndBack(test *testing.T) {
	tx1 := SampleTransaction()
	tx1.CallInfo.DeployedAddress = nil
	checkTransactionToAndFromProtobuf(test, tx1)

	// Check that pointer arguments work too, however, since we do pointer to values on convertion, do not compare for equality.
	tx2 := tx1
	event := tx2.CallInfo.Events[1]
	event.Arguments = append(event.Arguments, ToArguments(big.NewInt(123456), ToAddressPtr("*Address"), &eth.Hash{})...)

	checkTransactionToAndFromProtobufNoCheck(test, tx2)
}

func TestTransactionToProtobufAndBackError(test *testing.T){
	errTx1 := SampleTransaction()
	errTx1.CallInfo.Method.Arguments[0] = ToArgument(struct{}{})
	checkTransactionToProtobufError(test, errTx1)

	errTx2 := SampleTransaction()
	errTx2.CallInfo.Method.Arguments[0] = ToArgument(nil)
	checkTransactionToProtobufError(test, errTx2)
}