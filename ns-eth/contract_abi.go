package nseth

import (
	"strings"
	"math/big"
	logger "log"
	"github.com/pkg/errors"
	"reflect"
	"fmt"

	gethabi "github.com/ethereum/go-ethereum/accounts/abi"
	geth "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"

	"github.com/Multy-io/Multy-back/common/eth"
)

const (
	smartContractCallSigSize = 4 // e.g. 0xa9059cbb as byte-string
	smartContractEventSigSize = geth.HashLength
)

var (
	erc20abi = readABI("ERC20", erc20abiJSON)
	erc721abi = readABI("ERC721", erc721abiJSON)
	indexedMethods = indexMethods(erc20abi, erc721abi)
	indexedEvents = indexEvents(erc20abi, erc721abi)
)

type qualifiedMethod struct {
	ABI     annotatedABI
	Method  gethabi.Method
}

type qualifiedMethodsMap map[string]qualifiedMethod

func indexMethods(abis ...annotatedABI) qualifiedMethodsMap {
	methods := qualifiedMethodsMap{}

	for _, abi := range abis {
		for _, method := range abi.ABI.Methods {
			// We don't care about collisions from different contracts, since
			// same signature means same method name and same arguments.
			methods[string(method.Id())] = qualifiedMethod{
				ABI: abi,
				Method: method,
			}
		}
	}

	return methods
}

func indexEvents(abis ...annotatedABI) qualifiedEventsMap {
	events := qualifiedEventsMap{}

	for _, abi := range abis {
		for _, event := range abi.ABI.Events {
			// We don't care about collisions from different contracts, since
			// same signature means same event name and same arguments.
			events[string(event.Id().Bytes())] = qualifiedEvent{
				ABI: abi,
				Event: event,
			}
		}
	}

	return events
}
type qualifiedEvent struct {
	ABI     annotatedABI
	Event   gethabi.Event
}

type qualifiedEventsMap map[string]qualifiedEvent

type annotatedABI struct {
	ABI gethabi.ABI
	Description string
}

func readABI(description, json string) annotatedABI {
	abi, err := gethabi.JSON(strings.NewReader(json))
	if err != nil {
		logger.Panicf("Unexpected error while obtaining %s contract ABI: %#v", description, err)
	}

	return annotatedABI{
		ABI: abi,
		Description: description,
	}
}

func DecodeSmartContractCall(input string, address eth.Address) (*eth.SmartContractMethodInfo, error) {
	if len(input) < smartContractCallSigSize * 2 {
		return nil, errors.Errorf("Input is to small for smart contract call")
	}

	inputBytes, err := hexutil.Decode(input)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode input string from hex")
	}

	sig := inputBytes[:smartContractCallSigSize]

	var method qualifiedMethod
	var ok bool
	if method, ok = indexedMethods[string(sig)]; !ok {
		return nil, errors.Errorf("Unknown method signature: %s", hexutil.Encode(sig))
	}
	
	arguments, err := method.Method.Inputs.UnpackValues(inputBytes[smartContractCallSigSize:])
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to unpack method arguments %s", method.Method.Name)
	}

	result := &eth.SmartContractMethodInfo{
		Address: address,
		Name: method.Method.Sig(),
		Arguments: make([]eth.SmartContractMethodArgument, 0, len(arguments)),
	}

	for i, arg := range arguments {
		argDescription := method.Method.Inputs[i]
		typeName := argDescription.Type.String()

		value := convertType(arg)
		if value == nil {
			return nil, errors.Errorf("Failed to decode argument #%d %s %s of go-type: %v",
					i, argDescription.Name, typeName, reflect.TypeOf(arg))
		}

		result.Arguments = append(result.Arguments, eth.SmartContractMethodArgument(value))
	}

	return result, nil
}

func DecodeSmartContractEvent(input string, address eth.Address) (*eth.SmartContractEventInfo, error) {
	if len(input) < smartContractEventSigSize * 2 {
		return nil, errors.Errorf("Input is to small for smart contract event")
	}

	inputBytes, err := hexutil.Decode(input)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode input string from hex")
	}

	sig := inputBytes[:smartContractEventSigSize]

	var event qualifiedEvent
	var ok bool
	if event, ok = indexedEvents[string(sig)]; !ok {
		return nil, errors.Errorf("Unknown event signature: %s", hexutil.Encode(sig))
	}
	
	// HACK to force ABI into parsing all event arguments,
	// since it parses only non-indexed onces,
	// but inputBytes contains both indexed and non-indexed.
	inputs := make(gethabi.Arguments, 0, len(event.Event.Inputs))
	for _, input := range event.Event.Inputs {
		newInput := input
		newInput.Indexed = false

		inputs = append(inputs, newInput)
	}

	arguments, err := inputs.UnpackValues(inputBytes[smartContractEventSigSize:])
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to decode event arguments %s", event.Event.Name)
	}

	result := &eth.SmartContractEventInfo{
		Address: address,
		Name: eventName(&event.Event),
		Arguments: make([]eth.SmartContractEventArgument, 0, len(arguments)),
	}

	for i, arg := range arguments {
		input := event.Event.Inputs[i]
		typeName := input.Type.String()

		value := convertType(arg)
		if value == nil {
			return nil, errors.Errorf("Failed to decode argument #%d %s %s of go-type: %v",
					i, input.Name, typeName, reflect.TypeOf(arg))
		}

		result.Arguments = append(result.Arguments, eth.SmartContractEventArgument(value))
	}

	return result, nil
}

func eventName(event *gethabi.Event) string {
	// based on Event.Id() from geth
	types := make([]string, len(event.Inputs))
	i := 0
	for _, input := range event.Inputs {
		types[i] = input.Type.String()
		i++
	}

	return fmt.Sprintf("%v(%v)", event.Name, strings.Join(types, ","))
}

func convertType(value interface{}) interface{} {
	var result interface{}

	switch v := value.(type) {
	case int8:
		result = *big.NewInt(int64(v))
	case int16:
		result = *big.NewInt(int64(v))
	case int32:
		result = *big.NewInt(int64(v))
	case int64:
		result = *big.NewInt(int64(v))
	case uint8:
		result = *new(big.Int).SetUint64(uint64(v))
	case uint16:
		result = *new(big.Int).SetUint64(uint64(v))
	case uint32:
		result = *new(big.Int).SetUint64(uint64(v))
	case uint64:
		result = *new(big.Int).SetUint64(uint64(v))
	case *big.Int:
		result = *new(big.Int).Set(v)
	case big.Int:
		result = *new(big.Int).Set(&v)
	case geth.Address:
		result = eth.Address(v)
	case string:
		result = v
	case bool:
		result = v
	}

	return result
}

const erc20abiJSON = `[
	{"name": "approve", "type": "function", "constant": false, "inputs": [{"name": "spender", "type": "address"},{"name": "tokens", "type": "uint256"}], "outputs": [{"name": "success", "type": "bool"}], "payable": false, "stateMutability": "nonpayable"},
	{"name": "totalSupply", "type": "function", "constant": true, "inputs": [], "outputs": [{"name": "", "type": "uint256"}], "payable": false, "stateMutability": "view"},
	{"name": "transferFrom", "type": "function", "constant": false, "inputs": [{"name": "from", "type": "address"},{"name": "to", "type": "address"},{"name": "tokens", "type": "uint256"}], "outputs": [{"name": "success", "type": "bool"}], "payable": false, "stateMutability": "nonpayable"},
	{"name": "balanceOf", "type": "function", "constant": true, "inputs": [{"name": "tokenOwner", "type": "address"}], "outputs": [{"name": "balance", "type": "uint256"}], "payable": false, "stateMutability": "view"},
	{"name": "transfer", "type": "function", "constant": false, "inputs": [{"name": "to", "type": "address"},{"name": "tokens", "type": "uint256"}], "outputs": [{"name": "success", "type": "bool"}], "payable": false, "stateMutability": "nonpayable"},
	{"name": "allowance", "type": "function", "constant": true, "inputs": [{"name": "tokenOwner", "type": "address"},{"name": "spender", "type": "address"}], "outputs": [{"name": "remaining", "type": "uint256"}], "payable": false, "stateMutability": "view"},
	{"name": "Transfer", "type": "event", "anonymous": false, "inputs": [{"indexed": true, "name": "from", "type": "address"},{"indexed": true, "name": "to", "type": "address"},{"indexed": false, "name": "tokens", "type": "uint256"}]},
	{"name": "Approval", "type": "event", "anonymous": false, "inputs": [{"indexed": true, "name": "tokenOwner", "type": "address"},{"indexed": true, "name": "spender", "type": "address"},{"indexed": false, "name": "tokens", "type": "uint256"}]}
]`

const erc721abiJSON = `[
	{"name": "getApproved", "type": "function", "constant": true, "inputs": [{"name": "_tokenId", "type": "uint256"}], "outputs": [{"name": "", "type": "address"}], "payable": false, "stateMutability": "view"},
	{"name": "approve", "type": "function", "constant": false, "inputs": [{"name": "_approved", "type": "address"},{"name": "_tokenId", "type": "uint256"}], "outputs": [], "payable": true, "stateMutability": "payable"},
	{"name": "transferFrom", "type": "function", "constant": false, "inputs": [{"name": "_from", "type": "address"},{"name": "_to", "type": "address"},{"name": "_tokenId", "type": "uint256"}], "outputs": [], "payable": true, "stateMutability": "payable"},
	{"name": "safeTransferFrom", "type": "function", "constant": false, "inputs": [{"name": "_from", "type": "address"},{"name": "_to", "type": "address"},{"name": "_tokenId", "type": "uint256"}], "outputs": [], "payable": true, "stateMutability": "payable"},
	{"name": "ownerOf", "type": "function", "constant": true, "inputs": [{"name": "_tokenId", "type": "uint256"}], "outputs": [{"name": "", "type": "address"}], "payable": false, "stateMutability": "view"},
	{"name": "balanceOf", "type": "function", "constant": true, "inputs": [{"name": "_owner", "type": "address"}], "outputs": [{"name": "", "type": "uint256"}], "payable": false, "stateMutability": "view"},
	{"name": "setApprovalForAll", "type": "function", "constant": false, "inputs": [{"name": "_operator", "type": "address"},{"name": "_approved", "type": "bool"}], "outputs": [], "payable": false, "stateMutability": "nonpayable"},
	{"name": "safeTransferFrom", "type": "function", "constant": false, "inputs": [{"name": "_from", "type": "address"},{"name": "_to", "type": "address"},{"name": "_tokenId", "type": "uint256"},{"name": "data", "type": "bytes"}], "outputs": [], "payable": true, "stateMutability": "payable"},
	{"name": "isApprovedForAll", "type": "function", "constant": true, "inputs": [{"name": "_owner", "type": "address"},{"name": "_operator", "type": "address"}], "outputs": [{"name": "", "type": "bool"}], "payable": false, "stateMutability": "view"},
	{"name": "Transfer", "type": "event", "anonymous": false, "inputs": [{"indexed": true, "name": "_from", "type": "address"},{"indexed": true, "name": "_to", "type": "address"},{"indexed": true, "name": "_tokenId", "type": "uint256"}]},
	{"name": "Approval", "type": "event", "anonymous": false, "inputs": [{"indexed": true, "name": "_owner", "type": "address"},{"indexed": true, "name": "_approved", "type": "address"},{"indexed": true, "name": "_tokenId", "type": "uint256"}]},
	{"name": "ApprovalForAll", "type": "event", "anonymous": false, "inputs": [{"indexed": true, "name": "_owner", "type": "address"},{"indexed": true, "name": "_operator", "type": "address"},{"indexed": false, "name": "_approved", "type": "bool"}]}
]`