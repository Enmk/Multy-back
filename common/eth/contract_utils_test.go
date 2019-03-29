package eth

import (
	"testing"

	"math/big"

	. "github.com/Multy-io/Multy-back/tests"
)

var (
	expectedAddress = ToAddress("method argument address")
	expectedBigInt = *big.NewInt(123)
	expectedString = "method argument string"
	expectedHash = ToHash("method argument hash")
	expectedBool = true

	sampleMethod = SmartContractMethodInfo{
		Address: ToAddress("SC call address"),
		Name: "mock method",
		Arguments: ToArguments(
			expectedBigInt,
			expectedAddress,
			expectedString,
			expectedHash,
			expectedBool,
		),
	}
)

func TestUnpackMethodArguments(test *testing.T) {

	method := sampleMethod

	var allArguments struct {
		BigInt big.Int
		Address Address
		String string
		Hash Hash
		Bool bool
	}

	// it is ok to unpack only some of the arguments
	err := method.UnpackArguments(&allArguments)
	if err != nil {
		test.Fatalf("Failed to unpack arguments: %+v", err)
	}
	AssertEqual(test, expectedBigInt, allArguments.BigInt)
	AssertEqual(test, expectedAddress, allArguments.Address)
	AssertEqual(test, expectedString, allArguments.String)
	AssertEqual(test, expectedHash, allArguments.Hash)
	AssertEqual(test, expectedBool, allArguments.Bool)

	// some arguments may be present as interface{}
	var argumentsWithInterface struct {
		BigInt interface{}
		Address Address
		String string
		Hash Hash
		Bool bool
	}
	err = method.UnpackArguments(&argumentsWithInterface)
	if err != nil {
		test.Fatalf("Failed to unpack arguments: %+v", err)
	}
	if v, ok := argumentsWithInterface.BigInt.(big.Int); !ok {
		test.Fatalf("Failed to case field of interface type to actual value type (%T) => (%T)", argumentsWithInterface.BigInt, v)
	}
	AssertEqual(test, expectedBigInt, argumentsWithInterface.BigInt)
	AssertEqual(test, expectedAddress, argumentsWithInterface.Address)
	AssertEqual(test, expectedString, argumentsWithInterface.String)
	AssertEqual(test, expectedHash, argumentsWithInterface.Hash)
	AssertEqual(test, expectedBool, argumentsWithInterface.Bool)

	var partialArguments struct {
		BigInt big.Int
	}
	err = method.UnpackArguments(&partialArguments)
	if err != nil {
		test.Fatalf("Failed to unpack arguments: %+v", err)
	}
	AssertEqual(test, expectedBigInt, partialArguments.BigInt)

	// Converting to Amount from big.Int does not work for now.
	// var convertibleArguments struct {
	// 	Amount Amount
	// }
	// err = method.UnpackArguments(&convertibleArguments)
	// if err != nil {
	// 	test.Fatalf("Failed to unpack arguments: %+v", err)
	// }
	// AssertEqual(test, expectedBigInt, convertibleArguments.Amount.Int)
}

func TestUnpackMethodArgumentsError(test *testing.T) {
	method := sampleMethod

	var arguments struct {
		Address     Address
	}
	// Unpacking not to a pointer, but to value
	err := method.UnpackArguments(arguments)
	if err == nil {
		test.Fatalf("Unpacking expected to fail, instead unpacked to : %#v", arguments)
	}

	// Not the same type
	var invalidArguments struct {
		Int     int64
	}
	err = method.UnpackArguments(&invalidArguments)
	if err == nil {
		test.Fatalf("Unpacking expected to fail, instead unpacked to : %#v", invalidArguments)
	}

	// Not the same type
	var invalidArguments2 struct {
		Address     []byte
	}
	err = method.UnpackArguments(&invalidArguments2)
	if err == nil {
		test.Fatalf("Unpacking expected to fail, instead unpacked to : %#v", invalidArguments2)
	}

	// To many to unpack
	var toManyArgumentsArguments struct {
		Address Address
		Int big.Int
		String string
		hash Hash
		Bool bool
		Address2 Address
	}

	err = method.UnpackArguments(&toManyArgumentsArguments)
	if err == nil {
		test.Fatalf("Unpacking expected to fail, instead unpacked to : %#v", toManyArgumentsArguments)
	}
}