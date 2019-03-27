package ethtests

import (
	"math/big"
	
	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/common/eth"
)

func ToTxHash(str string) eth.TransactionHash {
	result := eth.TransactionHash{}
	copy(result[:], []byte(str))
	return result
}

func ToBlockHash(str string) eth.BlockHash {
	result := eth.BlockHash{}
	copy(result[:], []byte(str))
	return result
}

func ToAddress(str string) eth.Address {
	result := eth.Address{}
	copy(result[:], []byte(str))
	return result
}

func ToArgument(value interface{}) eth.SmartContractMethodArgument {
	return eth.SmartContractMethodArgument{Value:value}
}

// newBigIntFromHex panics on error
func NewBigIntFromHex(hexValue string) *big.Int {
	result, ok := new(big.Int).SetString(hexValue, 16)
	if !ok {
		panic(errors.Errorf("Faield to decode hex-encoded big.Int from  %s", hexValue))
	}

	return result
}

func ToAddressPtr(address string ) *eth.Address {
	addr := ToAddress(address)
	return &addr
}

func ToArguments(values ...interface{}) []eth.SmartContractMethodArgument {
	result := make([]eth.SmartContractMethodArgument, 0, len(values))
	for _, val := range values {
		result = append(result, eth.SmartContractMethodArgument{Value: val})
	}
	return result
}

// SampleTransaction returns sample transaction with all fields set with some predefined arbitrary data.
func SampleTransaction() eth.Transaction {
	// NOTE: setting pointer-arguments works too
	// but converting from protobuf to eth Transaction produces value-arguments.
	// so comparison would fail.
	return eth.Transaction{
		Hash:     ToTxHash("mock transaction hash"),
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
			DeployedAddress: ToAddressPtr("mock contract deployed address"),
			Method: &eth.SmartContractMethodInfo{
				Address: ToAddress("SC call address"),
				Name: "mock method",
				Arguments: ToArguments(
					ToAddress("method argument address"),
					*big.NewInt(123),
				),
			},
			Events: []eth.SmartContractEventInfo{
				{
					Address: ToAddress("SC event address 1"),
					Name: "mock event1",
					Arguments: ToArguments(
						ToAddress("event argument address"),
					),
				},
				{
					Address: ToAddress("SC event address 2"),
					Name: "mock event2",
					Arguments: ToArguments(
						ToAddress("event argument address"),
						"string value argument 1",
						*big.NewInt(1016),
						ToTxHash("mock tx hash"),
						false,
						true,
					),
				},
			},
		},
	}
}