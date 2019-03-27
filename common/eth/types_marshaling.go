package eth

import (
	"math/big"
	"encoding/json"

	"github.com/pkg/errors"

	"gopkg.in/mgo.v2/bson"
)

func (a *Amount) SetBSON(raw bson.Raw) error {
	var amountString string
	err := raw.Unmarshal(&amountString)
	if err != nil {
		return errors.Wrap(err, "Faield to parse amount from BSON")
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

type marshaler interface {
	Marshal(interface{}) ([]byte, error)
}

type MarshalerFunc func(interface{}) ([]byte, error)
func (f MarshalerFunc) Marshal(arg interface{}) ([]byte, error) {
    return f(arg)
}

type unmarshaler interface {
	Unmarshal([]byte, interface{}) error
}
type UnmarshalerFunc func([]byte, interface{}) error
func (f UnmarshalerFunc) Unmarshal(data []byte, i interface{}) error {
    return f(data, i)
}

func MarshalArgument(arg SmartContractMethodArgument, m marshaler) ([]byte, error) {
	return marshalArgumentValue(arg.Value, m)
}

func UnmarshalArgument(value []byte, u unmarshaler) (*SmartContractMethodArgument, error) {
	val, err := unmarshalArgumentValue(value, u)
	if err != nil {
		return nil, err
	}

	return &SmartContractMethodArgument{
		Value: val,
	}, nil
}

func marshalArgumentValue(arg interface{}, m marshaler) ([]byte, error) {
	value, err := m.Marshal(arg)
	typeByte := make([]byte, 1)

	switch v := arg.(type) {
	case Address, *Address:
		typeByte[0] = 'a'
	case *big.Int:
		typeByte[0] = 'i'
	case big.Int:
		typeByte[0] = 'i'
		value, err = m.Marshal(&v) // HACK: big.Int can only be marshalled from pointer
	case string:
		typeByte[0] = 's'
		value, err = m.Marshal(arg)
	case bool:
		typeByte[0] = 'b'
	case Hash, *Hash:
		typeByte[0] = 'h'
	default:
		return []byte{}, errors.Errorf("unknown argument type: %t", arg)
	}

	if err != nil {
		return []byte{}, errors.Wrapf(err, "Failed to marshal %t", arg)
	}

	value = append(typeByte, value...)
	return value, nil
}

func unmarshalArgumentValue(value []byte, u unmarshaler) (interface{}, error) {
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
		err = u.Unmarshal(data, &a)
		result = a
	case 'i':
		i := new(big.Int)
		err = u.Unmarshal(data, i)
		result = *i
	case 's':
		s := string("")
		err = u.Unmarshal(data, &s)
		result = s
	case 'b':
		b := bool(false)
		err = u.Unmarshal(data, &b)
		result = b
	case 'h':
		h := Hash{}
		err = u.Unmarshal(data, &h)
		result = h
	default:
		return nil, errors.Errorf("unknown argument type prefix '%c'", t)
	}

	return result, err
}

func (a *SmartContractMethodArgument) SetBSON(raw bson.Raw) error {
	value, err := unmarshalArgumentValue(raw.Data, UnmarshalerFunc(json.Unmarshal))
	if err != nil {
		return err
	}
	if value == nil {
		return errors.Errorf("Unmarshalled SmartContractMethodArgument is null")
	}

	a.Value = value
	return nil
}

func (a SmartContractMethodArgument) GetBSON() (interface{}, error) {
	return marshalArgumentValue(a.Value, MarshalerFunc(json.Marshal))
}
