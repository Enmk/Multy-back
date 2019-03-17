package ethtests

import (
	"math/big"
	
	"github.com/pkg/errors"

	"github.com/Multy-io/Multy-back/types/eth"
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

// newBigIntFromHex panics on error
func NewBigIntFromHex(hexValue string) *big.Int {
	result, ok := new(big.Int).SetString(hexValue, 16)
	if !ok {
		panic(errors.Errorf("Faield to decode hex-encoded big.Int from  %s", hexValue))
	}

	return result
}