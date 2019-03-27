package eth

import (
	"math/big"
	"strings"
	"time"

	"github.com/pkg/errors"

	geth "github.com/ethereum/go-ethereum/common"
)

const GWei = 1000 * 1000 * 1000

// Hash of the Block, often used as ID
type BlockHash = geth.Hash

// Hash of the Transaction, often used as ID
type TransactionHash = geth.Hash

// Blockchain address of the sender/recepient 
type Address = geth.Address

type Hash = geth.Hash

type RawTransaction string

type TransactionPayload []byte

type TransactionNonce uint64

type Amount struct {
	// surprisingly hard to work with type aliases, so we use a nested type to retain all methods of big.Int 
	big.Int
}

type AddressInfo struct {
	TotalBalance   Amount
	PendingBalance Amount
	Nonce          TransactionNonce
}

// Transaction is an Ethereun blockchain transaction

type Transaction struct {
	Hash     TransactionHash			//`json:"_id" bson:"_id"`
	Sender   Address					//`json:"sender" bson:"sender"`
	Receiver Address					//`json:"receiver" bson:"receiver"`
	Payload  TransactionPayload			//`json:"payload" bson:"payload"`
	Amount   Amount						//`json:"amount" bson:"amount"`
	Nonce    TransactionNonce			//`json:"nonce" bson:"nonce"`
	Fee      TransactionFee				//`json:"fee" bson:"fee"`
	CallInfo *SmartContractCallInfo		//`json:"call_info,omitempty" bson:"call_info,omitempty"`
	BlockInfo *TransactionBlockInfo		//`json:"block,omitempty" bson:"block,omitempty"`
}

type TransactionBlockInfo struct{
	Hash   BlockHash		//`json:"hash" bson:"hash"`
	Height uint64			//`json:"height" bson:"height"`
	Time   time.Time		//`json:"time" bson:"time"`
}

type GasLimit uint64
type GasPrice uint64 // up to 19 ETH for gas unit is more than enough

type TransactionFee struct {
	GasLimit GasLimit
	GasPrice GasPrice
}

type SmartContractCallInfo struct {
	// Status of the call
	Status SmartContractCallStatus		//`json:"status" bson:"status"`
	// Method that was called, maybe null if unknown or unable to parse, may be null.
	Method *SmartContractMethodInfo		//`json:"method" bson:"method"`
	// Events that were generated during execution, only non-removed events.
	Events []SmartContractEventInfo		//`json:"events" bson:"events"`
	// Address of new deployed contract, may be null
	DeployedAddress *Address			//`json:"deployed_address" bson:"deployed_address"`
}

type SmartContractMethodInfo struct {
	Address Address								//`json:"address" bson:"address"`
	Name string									//`json:"name" bson:"name"`
	Arguments []SmartContractMethodArgument		//`json:"arguments" bson:"arguments"`
}

type SmartContractMethodArgument struct {
	Value interface{}
}
type SmartContractEventInfo = SmartContractMethodInfo
type SmartContractEventArgument = SmartContractMethodArgument

type SmartContractCallStatus int

const (
	SmartContractCallStatusOk SmartContractCallStatus = 1
	SmartContractCallStatusFailed SmartContractCallStatus = 0
)

type TransactionWithStatus struct {
	Transaction                   `json:",inline" bson:",inline"`
	Status      TransactionStatus //`json:"status" bson:"status"`
}

// Positive values mean non-error statuses, Negative mean different error conditions
type TransactionStatus int

const (
	/// Happy path (usually transaction status should change to 1 => 2 => 3)

	// TransactionStatusInMempool is for transactions in mempool.
	TransactionStatusInMempool			TransactionStatus = 1

	// TransactionStatusInBlock is for transactions in block on
	// canonical chain, which is not yet considered immutable.
	TransactionStatusInBlock			TransactionStatus = 2

	// TransactionStatusInImmutableBlock is for transactions in block on
	// canonical chain, which is old enought to be considered immutable.
	// At this point, there should be no changes in transaction status.
	TransactionStatusInImmutableBlock	TransactionStatus = 3

	/// Errors

	// TransactionStatusError is a generic error.
	TransactionStatusError 				TransactionStatus = -1

	// TransactionStatusErrorRejected happens only for transactions
	// that were rejected by the node, i.e. it can never appear neither
	// in mempool nor in block, sicne it is considered malformed.
	TransactionStatusErrorRejected 		TransactionStatus = -2

	// TransactionStatusErrorReplaced occurs when
	// another transaction with the same nonce from this address
	// was included in the block on canonical path and that block
	// is old enough to be considered immutable or final.
	TransactionStatusErrorReplaced 		TransactionStatus = -3

	// TransactionStatusErrorSmartContractCallFailed occurs when
	// transaction was mined, but the SC call that was performed
	// by this transaction failed.
	TransactionStatusErrorSmartContractCallFailed TransactionStatus = -4

	// TransactionStatusErrorLost occurs when transaction was submitted to the node,
	// but dropped out of mempool and/or can't be found on canonical path.
	TransactionStatusErrorLost 		TransactionStatus = -5
)

// BlockHeader is a header of the Ethereum blockchain block
type BlockHeader struct {
	Hash   BlockHash	//`json:"hash" bson:"_id"`
	Height uint64		//`json:"height" bson:"height"`
	Parent BlockHash	//`json:"parent_id" bson:"parent_id"`
	Time   time.Time
}

// Block is an Ethereum blockchain block
type Block struct {
	BlockHeader  `json:",inline" bson:",inline"`
	Transactions []TransactionHash //`json:"transactions" bson:"transactions"`
}

func NewAmountFromInt64(value int64) *Amount {
	return &Amount{
		Int: *big.NewInt(value),
	}
}

func NewAmountFromString(str string, base int) (*Amount, error) {
	amount := &Amount{}
	_, ok := amount.SetString(str, base)
	if ok == false {
		return nil, errors.Errorf("Failed to create an Amount from string: \"%s\" with base: %d", str, base)
	}

	return amount, nil
}

func HexToHash(hexString string) Hash {
	return geth.HexToHash(hexString)
}

// HexToAddress converts hex-encoded string (it may be prefixed with 0x) to Address.
func HexToAddress(hexString string) Address {
	return Address(geth.HexToAddress(hexString))
}

// HexToAmount converts hex-encoded string (it may be prefixed with 0x) to Amount.
func HexToAmount(hexString string) (Amount, error) {
	if strings.HasPrefix(hexString, "0x") {
		hexString = hexString[2:]
	}
	if hexString == "" {
		return Amount{
			Int: *big.NewInt(0),
		}, nil
	}

	value, ok := new(big.Int).SetString(hexString, 16)
	if !ok {
		return Amount{}, errors.Errorf("Faield to create eth.Amount from hex string: %s", hexString)
	}

	return Amount{
		Int: *value,
	}, nil
}

func (amount Amount) Hex() string {
	return "0x" + amount.Text(16)
}
