package tests

import (
	"reflect"
	"os"
	"testing"
	"runtime"

	"github.com/davecgh/go-spew/spew"
)

func getCaller() runtime.Frame {
	pc := make([]uintptr, 15)
	n := runtime.Callers(3, pc) // 3 is to skip runtime.Callers(), getCaller(), and  caller of getCaller()
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	// fmt.Printf("!!! %s,:%d %s\n", frame.File, frame.Line, frame.Function)
	return frame
}

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
		caller := getCaller()
		test.Errorf("%s:%d (%s) : Assertion failed: expected != actual\nexpected:\n%s\nactual:\n%s",
				caller.File, caller.Line, caller.Function, l, r)
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