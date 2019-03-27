package tests

import (
	"reflect"
	"os"
	"testing"

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

func AssertEqual(test *testing.T, left, right interface{}) bool {
	if equal, l, r := TestEqual(left, right); !equal {
		test.Errorf("Assertion failed: expected != actual\nexpected:\n%s\nactual:\n%s", l, r)
		return false
	}

	return true
}

func GetenvOrDefault(key, defaultValue string) (result string) {
	result, ok := os.LookupEnv(key)
	if !ok {
		result = defaultValue
	}

	return result
}