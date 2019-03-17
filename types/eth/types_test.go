package eth

import (
	"testing"
)

func checkHexToAmount(test *testing.T, input string, expectedValue int64) {
	amount, err := HexToAmount(input)
	if err != nil {
		test.Errorf("failed to parse hex-encoded amount: %+v", err)
	}

	if amount.Int64() != expectedValue {
		test.Errorf("expected %#v == %v", amount, expectedValue)
	}
}

func checkHexToAddress(test *testing.T, input string, expectedAddress Address) {
	address := HexToAddress(input)
	if address != expectedAddress {
		test.Errorf("expected %#v == %#v", address, expectedAddress)
	}
}

func TestHexHashToAmount(test *testing.T) {
	const expectedAmount int64 = 0x43a1

	checkHexToAmount(test, "0x00000000000000000000000000000000000000000000000000000000000043a1", expectedAmount)
	checkHexToAmount(test, "00000000000000000000000000000000000000000000000000000000000043a1", expectedAmount)
	checkHexToAmount(test, "0x43a1", expectedAmount)
	checkHexToAmount(test, "43a1", expectedAmount)
}

func TestHexHashToAddress(test *testing.T) {

	expectedAddress := toAddress("\xe7\x23\x2a\x9f\xd8\xbf\x42\x7a\xa4\x19\x18\xbc\x00\x8d\x32\x29\x0e\x22\x99\x0e")

	checkHexToAddress(test, "0x000000000000000000000000e7232a9fd8bf427aa41918bc008d32290e22990e", expectedAddress)
	checkHexToAddress(test, "e7232a9fd8bf427aa41918bc008d32290e22990e", expectedAddress)
	checkHexToAddress(test, "0xe7232a9fd8bf427aa41918bc008d32290e22990e", expectedAddress)
}

func toAddress(str string) Address {
	result := Address{}
	copy(result[:], []byte(str))
	return result
}
