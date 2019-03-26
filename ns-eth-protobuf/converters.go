package nsethprotobuf

import (
	"encoding/json"
	"math/big"
	"github.com/pkg/errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/Multy-io/Multy-back/common/eth"
)

type converterError struct {
	error
}

func TransactionToProtobuf(transaction eth.Transaction) (result *ETHTransaction, err error) {
	// Handle internal panic and return as error, just as json does.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(converterError); ok {
				err = e.error
			} else {
				panic(r)
			}
		}
	}()

	result = &ETHTransaction{
		Hash:     transaction.ID.Hex(),
		From:     transaction.Sender.Hex(),
		To:       transaction.Receiver.Hex(),
		Amount:   transaction.Amount.Hex(),
		GasPrice: uint64(transaction.Fee.GasPrice),
		GasLimit: uint64(transaction.Fee.GasLimit),
		Nonce:    uint64(transaction.Nonce),
		// TODO: change type in pb.ETHTransaction to bytes
		Payload:  hexutil.Encode(transaction.Payload),
	}

	if callInfo := transaction.CallInfo; callInfo != nil {

		var deployedAddress string
		if address := transaction.CallInfo.DeployedAddress; address != nil {
			deployedAddress = address.Hex()
		}

		events := make([]*SmartContractCall, 0, len(callInfo.Events))
		for _, event := range callInfo.Events {
			events = append(events, SmartContractMethodInfoToProtobuf(&event))
		}

		result.ContractInfo = &SmartContractInfo{
			Status: int32(callInfo.Status),
			Method: SmartContractMethodInfoToProtobuf(callInfo.Method),
			DeployedAddress: deployedAddress,
			Events: events,
		}
	}

	return result, nil
}

func TransactionFromProtobuf(transaction ETHTransaction) (result *eth.Transaction, err error) {
	// Handle internal panic and return as error, just as json does.
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(converterError); ok {
				err = e.error
			} else {
				panic(r)
			}
		}
	}()

	amount, err := eth.HexToAmount(transaction.Amount)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to convert Transaction.Amount from protobuf")
	}

	payload, err := hexutil.Decode(transaction.Payload)
	if err != nil {
		return nil, errors.WithMessage(err, "Failed to convert Transaction.Payload from protobuf")
	}

	result = &eth.Transaction{
		ID:       common.HexToHash(transaction.Hash),
		Sender:   eth.HexToAddress(transaction.From),
		Receiver: eth.HexToAddress(transaction.To),
		Payload:  payload,
		Amount:   amount,
		Fee:      eth.TransactionFee{
			GasPrice: eth.GasPrice(transaction.GasPrice),
			GasLimit: eth.GasLimit(transaction.GasLimit),
		},
		Nonce:    eth.TransactionNonce(transaction.Nonce),
	}

	if contractInfo := transaction.ContractInfo; contractInfo != nil {
		var deployedAddress *eth.Address
		if hexAddress := contractInfo.DeployedAddress; hexAddress != "" {
			addr := eth.HexToAddress(hexAddress)
			deployedAddress = &addr
		}

		events := make([]eth.SmartContractEventInfo, 0, len(contractInfo.Events))
		for _, event := range contractInfo.Events {
			newEvent := SmartContractMethodInfoFromProtobuf(event)
			if newEvent == nil {
				panic(converterError{errors.Errorf("event is not supposed to be nil")})
			}

			events = append(events, *newEvent)
		}

		result.CallInfo = &eth.SmartContractCallInfo{
			Status: eth.SmartContractCallStatus(contractInfo.Status),
			Method: SmartContractMethodInfoFromProtobuf(contractInfo.Method),
			Events: events,
			DeployedAddress: deployedAddress,
		}
	}

	return result, nil
}

func SmartContractMethodInfoToProtobuf(methodInfo *eth.SmartContractMethodInfo) *SmartContractCall {
	if methodInfo == nil {
		return nil
	}

	address := Address{
		Address: methodInfo.Address.Hex(),
	}
	result := &SmartContractCall{
		Address: &address,
		Name: methodInfo.Name,
	}

	arguments := make([][]byte, 0, len(methodInfo.Arguments))
	for i, arg := range methodInfo.Arguments {
		value, err := json.Marshal(arg)
		typeByte := make([]byte, 1)

		switch v := arg.(type) {
		case eth.Address, *eth.Address:
			typeByte[0] = 'a'
		case *big.Int:
			typeByte[0] = 'i'
		case big.Int:
			typeByte[0] = 'i'
			value, err = json.Marshal(&v)
		case string:
			typeByte[0] = 's'
		case bool:
			typeByte[0] = 'b'
		case eth.Hash, *eth.Hash:
			typeByte[0] = 'h'
		default:
			panic(converterError{errors.Errorf("unknown argument type #%d of '%s': %t",
					i, methodInfo.Name, arg)})
		}

		if err != nil {
			// This is a pretty severe errors, since all values should be marshallable.
			panic(converterError{errors.Wrapf(err,
					"failed to marshall argument #%d of '%s'",
					i, methodInfo.Name)})
		}

		value = append(typeByte, value...)
		arguments = append(arguments, value)
	}
	result.Arguments = arguments

	return result
}

func SmartContractMethodInfoFromProtobuf(callInfo *SmartContractCall) *eth.SmartContractMethodInfo {
	if callInfo == nil {
		return nil
	}

	arguments := make([]eth.SmartContractMethodArgument, 0, len(callInfo.Arguments))
	for i, arg := range callInfo.Arguments {
		var value interface{}
		if len(arg) == 0 {
			panic(converterError{errors.Errorf("not enough data to parse argument #%d of '%s'",
					i, callInfo.Name)})
		}

		t := arg[0]
		data := arg[1:]
		var err error

		switch t {
		case 'a':
			a := *new(eth.Address)
			err = json.Unmarshal(data, &a)
			value = a
		case 'i':
			i := new(big.Int)
			err = json.Unmarshal(data, i)
			value = *i
		case 's':
			s := string("")
			err = json.Unmarshal(data, &s)
			value = s
		case 'b':
			b := bool(false)
			err = json.Unmarshal(data, &b)
			value = b
		case 'h':
			h := eth.Hash{}
			err = json.Unmarshal(data, &h)
			value = h
		default:
			panic(converterError{errors.Errorf("unknown argument type #%d of '%s': %c",
					i, callInfo.Name, t)})
		}

		if err != nil {
			// This is a pretty severe errors, since all values should be marshallable.
			panic(converterError{errors.Wrapf(err,
					"failed to unmarshall argument #%d of '%s' into %+#v",
					i, callInfo.Name, value)})
		}

		arguments = append(arguments, value)
	}

	var address eth.Address
	if callInfo.Address != nil {
		address = eth.HexToAddress(callInfo.Address.Address)
	}

	return &eth.SmartContractMethodInfo{
		Address:   address,
		Name:      callInfo.Name,
		Arguments: arguments,
	}
}