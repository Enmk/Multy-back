package client

import (
	"testing"

	. "github.com/Multy-io/Multy-back/tests"

	"github.com/gin-gonic/gin"
)

type Params struct {
	Int     int     `json:"int"`
	Int8    int8    `json:"int8"`
	Int16   int16   `json:"int16"`
	Int32   int32   `json:"int32"`
	Int64   int64   `json:"int64"`
	Uint    uint    `json:"uint"`
	Uint8   uint8   `json:"uint8"`
	Uint16  uint16  `json:"uint16"`
	Uint32  uint32  `json:"uint32"`
	Uint64  uint64  `json:"uint64"`
	String  string  `json:"string"`
	Bool    bool    `json:"bool"`
	Float32 float32 `json:"float32"`
	Float64 float64 `json:"float64"`
}

var ginParams = gin.Params{
	gin.Param{
		"unboundparam0", "never used", // param that is not bound to value in struct
	},
	gin.Param{
		"int", "-1337",
	},
	gin.Param{
		"int8", "-123",
	},
	gin.Param{
		"int16", "-1234",
	},
	gin.Param{
		"int32", "-1234567",
	},
	gin.Param{
		"int64", "-1234567890",
	},
	gin.Param{
		"uint", "1337",
	},
	gin.Param{
		"uint8", "123",
	},
	gin.Param{
		"uint16", "1234",
	},
	gin.Param{
		"unboundparam1", "never used too", // param that is not bound to value in struct
	},
	gin.Param{
		"uint32", "1234567",
	},
	gin.Param{
		"uint64", "1234567890",
	},
	gin.Param{
		"string", "string",
	},
	gin.Param{
		"bool", "true",
	},
	gin.Param{
		"float32", "0.1",
	},
	gin.Param{
		"float64", "0.5", // carfully chosen to have exact representation in float to simplify comparison
	},
	gin.Param{
		"unboundparam2", "0.5", // param that is not bound to value in struct
	},
}

func TestBindParams(test *testing.T) {
	params := ginParams

	expectedParams := Params{
		-1337,
		-123,
		-1234,
		-1234567,
		-1234567890,
		1337,
		123,
		1234,
		1234567,
		1234567890,
		"string",
		true,
		0.1,
		0.5,
	}

	paramsStruct := Params{}
	err := BindParams(params, &paramsStruct)
	if err != nil {
		test.Fatalf("Faild to bind values from params to struct: %+v", err)
	}

	AssertEqual(test, expectedParams, paramsStruct)
}

func TestBindParamsAnonymousStruct(test *testing.T) {
	params := ginParams

	// unexported anonymous struct with unexported parameters
	p := []struct {
		aInt     int     `json:"int"`
		aInt8    int8    `json:"int8"`
		aInt16   int16   `json:"int16"`
		aInt32   int32   `json:"int32"`
		aInt64   int64   `json:"int64"`
		aUint    uint    `json:"uint"`
		aUint8   uint8   `json:"uint8"`
		aUint16  uint16  `json:"uint16"`
		aUint32  uint32  `json:"uint32"`
		aUint64  uint64  `json:"uint64"`
		aString  string  `json:"string"`
		aBool    bool    `json:"bool"`
		aFloat32 float32 `json:"float32"`
		aFloat64 float64 `json:"float64"`
	}{{
		-1337,
		-123,
		-1234,
		-1234567,
		-1234567890,
		1337,
		123,
		1234,
		1234567,
		1234567890,
		"string",
		true,
		0.1,
		0.5,
	},
		{}}

	expectedParams := p[0]
	actualParams := p[1]

	// paramsStruct := Params{}
	err := BindParams(params, &actualParams)
	if err != nil {
		test.Fatalf("Failed to bind values from params to struct: %+v", err)
	}

	AssertEqual(test, expectedParams, actualParams)
}

func TestBindParamsOmitempty(test *testing.T) {
	params := ginParams

	p := []struct {
		afoo     int     `json:"foo,omitempty"`
		aInt     int     `json:"int"`
		aInt8    int8    `json:"int8"`
		aInt16   int16   `json:"int16"`
		aInt32   int32   `json:"int32"`
		aInt64   int64   `json:"int64"`
		aUint    uint    `json:"uint"`
		aUint8   uint8   `json:"uint8"`
		aUint16  uint16  `json:"uint16"`
		aUint32  uint32  `json:"uint32"`
		aUint64  uint64  `json:"uint64"`
		aString  string  `json:"string"`
		aBool    bool    `json:"bool"`
		aFloat32 float32 `json:"float32"`
		aFloat64 float64 `json:"float64"`
		abar     int     `json:"bar,omitempty"`
	}{{ // expected when all fields set
		999,
		-1337,
		-123,
		-1234,
		-1234567,
		-1234567890,
		1337,
		123,
		1234,
		1234567,
		1234567890,
		"string",
		true,
		0.1,
		0.5,
		666,
	},
		{}, // actual
		{ // expected when optional fields not set
			0,
			-1337,
			-123,
			-1234,
			-1234567,
			-1234567890,
			1337,
			123,
			1234,
			1234567,
			1234567890,
			"string",
			true,
			0.1,
			0.5,
			0,
		}}

	expectedAllParams := p[0]
	actualParams := p[1]
	expectedPartialParams := p[2]

	// not all values are present in params, enusre that binding works ok and fields not actually set
	err := BindParams(params, &actualParams)
	if err != nil {
		test.Fatalf("Failed to bind values from params to struct: %+v", err)
	}
	AssertEqual(test, expectedPartialParams, actualParams)

	// set missing fields, verify that it works
	params = append(params, gin.Param{"foo", "999"}, gin.Param{"bar", "666"})
	err = BindParams(params, &actualParams)
	if err != nil {
		test.Fatalf("Failed to bind values from params to struct: %+v", err)
	}
	AssertEqual(test, expectedAllParams, actualParams)
}
