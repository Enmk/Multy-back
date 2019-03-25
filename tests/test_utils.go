package tests

import (
	"reflect"
	"os"

	"github.com/davecgh/go-spew/spew"
)

// Utility to deeply complare values and provide string dump if they are not equal.
func TestEqual(left, right interface{}) (eq bool, leftS, rightS string) {
	eq = reflect.DeepEqual(left, right)
	if !eq {
		leftS = spew.Sdump(left)
		rightS = spew.Sdump(right)
	}

	return eq, leftS, rightS
}

func GetenvOrDefault(key, defaultValue string) (result string) {
	result, ok := os.LookupEnv(key)
	if !ok {
		result = defaultValue
	}

	return result
}