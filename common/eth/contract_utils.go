package eth

import (
	"reflect"
	"github.com/pkg/errors"
)

const Erc20TransferName = "transfer(address,uint256)"
// Signature of ERC20/721 `Transfer` event.
const TransferEventName = "Transfer(address,address,uint256)"

func (method *SmartContractMethodInfo) UnpackArguments(out interface{}) error {

	val := reflect.ValueOf(out)

	switch val.Kind() {
	case reflect.Ptr:
		val = reflect.Indirect(val)
	default:
		return errors.Errorf("Failed to unpack \"%s\" arguments: need a pointer to a struct.", method.Name)
	}

	if len(method.Arguments) < val.NumField() {
		return errors.Errorf("Failed to unpack \"%s\" arguments: requested to many values to unpack: %d, but only %d available",
				method.Name,
				val.NumField(),
				len(method.Arguments))
	}

	for i := 0; i < val.NumField(); i ++ {
		arg := method.Arguments[i]

		f := val.Field(i)
		argValue := reflect.ValueOf(arg.Value)

		if f.CanSet() && argValue.Type().AssignableTo(f.Type()) {
			f.Set(argValue)
		} else {
			return errors.Errorf("Failed to set \"%s\" argument value of type %s to field %s", method.Name, argValue.String(), f.String())
		}
	}

	return nil
}