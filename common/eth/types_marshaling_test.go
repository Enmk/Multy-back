package eth

import (
	"testing"
	"math/big"

	"gopkg.in/mgo.v2/bson"

	. "github.com/Multy-io/Multy-back/tests"
)

func ToTxHash(str string) TransactionHash {
	result := TransactionHash{}
	copy(result[:], []byte(str))
	return result
}

func ToHash(str string) Hash {
	result := Hash{}
	copy(result[:], []byte(str))
	return result
}

func ToBlockHash(str string) BlockHash {
	result := BlockHash{}
	copy(result[:], []byte(str))
	return result
}

func ToAddress(str string) Address {
	result := Address{}
	copy(result[:], []byte(str))
	return result
}

func ToArgument(value interface{}) SmartContractMethodArgument {
	return SmartContractMethodArgument{Value:value}
}

func ToArguments(values ...interface{}) []SmartContractMethodArgument {
	result := make([]SmartContractMethodArgument, 0, len(values))
	for _, val := range values {
		result = append(result, SmartContractMethodArgument{Value: val})
	}
	return result
}

func checkSmartContractArgumentBSON(test *testing.T, expectedArg SmartContractMethodArgument) {

	data, err := bson.Marshal(expectedArg)
	if err != nil {
		test.Errorf("Failed to marshal SmartContractMethodArgument to BSON: %+v", err)
		return
	}

	var actualArg SmartContractMethodArgument
	err = bson.Unmarshal(data, &actualArg)
	if err != nil {
		test.Errorf("Failed to unmarshal SmartContractMethodArgument from BSON: %s\nerror: %+v", data, err)
		return
	}

	if !AssertEqual(test, expectedArg, actualArg) {
		return
	}
}

func TestSmartContractArgumentToAndFromBSON(test *testing.T) {
	checkSmartContractArgumentBSON(test, ToArgument("simple string"))
	checkSmartContractArgumentBSON(test, ToArgument(false))
	checkSmartContractArgumentBSON(test, ToArgument(true))
	checkSmartContractArgumentBSON(test, ToArgument(*big.NewInt(1337)))
	checkSmartContractArgumentBSON(test, ToArgument(ToAddress("mock address")))
	checkSmartContractArgumentBSON(test, ToArgument(ToHash("mock hash")))
}