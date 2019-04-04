package client

import (
	"reflect"
	"strconv"
	"unsafe"
	"github.com/pkg/errors"

	"github.com/gin-gonic/gin"
)

func getNameFromTag(tagVal string) string {
	return tagVal
}

func canOmmit(tagVal string) bool {
	return false
}

func findValue(params gin.Params, key string) (string, error) {
	for _, p := range params {
		if p.Key == key {
			return p.Value, nil
		}
	}

	return "", errors.Errorf("Cound't find \"%s\" param", key)
}

func BindParams(params gin.Params, out interface{}) error {
	// read parameters from `params` to out as structure fields, honor struct field type and "omitempty" specifiers

	v := reflect.Indirect(reflect.ValueOf(out))
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("json")
		fv := v.Field(i)

		// Cheat to make unexported field writable https://stackoverflow.com/a/43918797
		fv = reflect.NewAt(fv.Type(), unsafe.Pointer(fv.UnsafeAddr())).Elem()
		if !fv.CanSet() {
			return errors.Errorf("Field \"%s\" is unsettable", field.Name)
		}

		paramName := getNameFromTag(tag)
		canOmit := canOmmit(tag)

		paramValue, err := findValue(params, paramName)
		if err != nil {
			if canOmit {
				continue
			}

			return errors.WithMessagef(err, "Can't find param value with name \"%s\" to set field \"%s\"", paramName, field.Name)
		}
	
		// declared here to avoid shadowing `err` value in strconv.ParseXXX below.
		var ival int64
		var uval uint64
		var bval bool
		var fval float64
		switch fv.Interface().(type) {
		case int:
			ival, err = strconv.ParseInt(paramValue, 10, 32)
			if err == nil {
				fv.SetInt(ival)
			}
		case int8:
			ival, err = strconv.ParseInt(paramValue, 10, 8)
			if err == nil {
				fv.SetInt(ival)
			}
		case int16:
			ival, err = strconv.ParseInt(paramValue, 10, 16)
			if err == nil {
				fv.SetInt(ival)
			}
		case int32:
			ival, err = strconv.ParseInt(paramValue, 10, 32)
			if err == nil {
				fv.SetInt(ival)
			}
		case int64:
			ival, err = strconv.ParseInt(paramValue, 10, 64)
			if err == nil {
				fv.SetInt(ival)
			}
		case uint:
			uval, err = strconv.ParseUint(paramValue, 10, 32)
			if err == nil {
				fv.SetUint(uval)
			}
		case uint8:
			uval, err = strconv.ParseUint(paramValue, 10, 8)
			if err == nil {
				fv.SetUint(uval)
			}
		case uint16:
			uval, err = strconv.ParseUint(paramValue, 10, 16)
			if err == nil {
				fv.SetUint(uval)
			}
		case uint32:
			uval, err = strconv.ParseUint(paramValue, 10, 32)
			if err == nil {
				fv.SetUint(uval)
			}
		case uint64:
			uval, err = strconv.ParseUint(paramValue, 10, 64)
			if err == nil {
				fv.SetUint(uval)
			}
		case string:
			fv.SetString(paramValue)
		case bool:
			bval, err = strconv.ParseBool(paramValue)
			if err == nil {
				fv.SetBool(bval)
			}
		case float32:
			fval, err = strconv.ParseFloat(paramValue, 32)
			if err == nil {
				fv.SetFloat(fval)
			}
		case float64:
			fval, err = strconv.ParseFloat(paramValue, 32)
			if err == nil {
				fv.SetFloat(fval)
			}
		default:
			return errors.Errorf("Can't set value to field %s of unsupported type: %s", field.Name, fv.Type().Name())
		}
	}

	return nil
}