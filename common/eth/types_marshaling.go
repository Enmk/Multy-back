package eth

import (
	"math/big"

	"github.com/pkg/errors"

	"gopkg.in/mgo.v2/bson"
)

func (a *Amount) SetBSON(raw bson.Raw) error {
	var amountString string
	err := raw.Unmarshal(&amountString)
	if err != nil {
		return errors.Wrap(err, "Failed to parse amount from BSON")
	}

	amount, err := HexToAmount(amountString)
	if err != nil {
		return err
	}
	*a = amount

	return nil
}

func (a Amount) GetBSON() (interface{}, error) {
	return a.Hex(), nil
}

func MarshalArgument(arg SmartContractMethodArgument) ([]byte, error) {
	typeByte := make([]byte, 1)
	var data []byte

	switch v := arg.Value.(type) {
	case Address:
		typeByte[0] = 'a'
		data = v.Bytes()
	case *Address:
		typeByte[0] = 'a'
		data = v.Bytes()
	case *big.Int:
		typeByte[0] = 'i'
		data = v.Bytes()
	case big.Int:
		typeByte[0] = 'i'
		data = v.Bytes()
	case string:
		typeByte[0] = 's'
		data = []byte(v)
	case bool:
		typeByte[0] = 'b'
		if v {
			data = []byte{1}
		} else {
			data = []byte{2}
		}
	case Hash:
		typeByte[0] = 'h'
		data = v.Bytes()
	case *Hash:
		typeByte[0] = 'h'
		data = v.Bytes()
	default:
		return []byte{}, errors.Errorf("unknown argument type: %t", arg)
	}

	return append(typeByte, data...), nil
}

func UnmarshalArgument(value []byte) (*SmartContractMethodArgument, error) {
	if len(value) == 0 {
		return nil, errors.Errorf("not enough data to parse argument value")
	}

	t := value[0]
	data := value[1:]

	var err error
	var result interface{}

	switch t {
	case 'a':
		a := *new(Address)
		a.SetBytes(data)
		result = a
	case 'i':
		i := new(big.Int)
		i.SetBytes(data)
		result = *i
	case 's':
		s := string(data)
		result = s
	case 'b':
		b := bool(false)
		if data[0] == 1 {
			b = true
		}
		result = b
	case 'h':
		h := Hash{}
		h.SetBytes(data)
		result = h
	default:
		return nil, errors.Errorf("unknown argument type prefix '%c' (%d)", t, int(t))
	}

	return &SmartContractMethodArgument{Value:result}, err
}

func (a *SmartContractMethodArgument) SetBSON(raw bson.Raw) error {
	var doc bson.M
	err := raw.Unmarshal(&doc)
	if err != nil {
		return errors.Wrapf(err, "Failed to unmarshal []byte from BSON")
	}

	v, ok := doc["value"]
	if !ok {
		return errors.Errorf("Invalid SmartContractMethodArgument document: missing value")
	}
	data, ok := v.([]byte)
	if !ok {
		return errors.Errorf("Invalid SmartContractMethodArgument document: value is in unsupported format")
	}

	value, err := UnmarshalArgument(data)
	if err != nil {
		return err
	}

	if value == nil {
		return errors.Errorf("Unmarshalled SmartContractMethodArgument is null")
	}

	*a = *value
	return nil
}

func (a SmartContractMethodArgument) GetBSON() (interface{}, error) {
	data, err := MarshalArgument(a)
	if err != nil {
		return nil, err
	}

	return bson.M{"value": data}, nil
}

// Marshalling tricks: since transaction stores Amount by-value, 
// big.Int marshalling is no applicable and Amount is marshalled as struct, merely to "Int: {}":
// https://github.com/golang/go/issues/28946#issuecomment-441684687

// MarshalJSON implements the json.Marshaler interface. 
func (amount Amount) MarshalJSON() ([]byte, error) {
	return []byte(amount.Hex()), nil
}

// UnmarshalJSON implements the json.Unmarshaler interface. 
func (amount *Amount) UnmarshalJSON(text []byte) error {
	newAmount, err := HexToAmount(string(text))
	if err != nil {
		return err
	}

	*amount = newAmount
	return nil
}
