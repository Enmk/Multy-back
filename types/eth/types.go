package eth

import (
	"math/big"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

// Hash of the Block, often used as ID
type BlockHash string

// Hash of the Transaction, often used as ID
type TransactionHash string

// Blockchain address of the sender/recepient 
type Address string

type TransactionPayload []byte

type TransactionNonce uint64

// surprisingly hard to work with type aliases
type Amount struct {
	big.Int
}

// Transaction is an Ethereun blockchain transaction

type Transaction struct {
	ID TransactionHash			`json:"_id" bson:"_id"`
	Sender Address				`json:"sender" bson:"sender"`
	Receiver Address			`json:"receiver" bson:"receiver"`
	Payload TransactionPayload	`json:"payload" bson:"payload"`
	Amount *Amount				`json:"amount" bson:"amount"`
	Nonce TransactionNonce		`json:"nonce" bson:"nonce"`
	Fee TransactionFee			`json:"fee" bson:"fee"`
}

type GasLimit uint64
type GasPrice uint64 // up to 19 ETH for gas is more than enough

type TransactionFee struct {
	GasLimit GasLimit
	GasPrice GasPrice
}

const GWei = 1000*1000*1000
type TransactionWithStatus struct {
	Transaction					`json:",inline" bson:",inline"`
	Status TransactionStatus	`json:"status" bson:"status"`
}

// Positive values mean non-error statuses, Negative mean different error conditions
type TransactionStatus int

const (
	// Happy path:
	TransactionStatusInMempool			TransactionStatus = 1
	TransactionStatusInBlock			TransactionStatus = 2
	TransactionStatusInImmutableBlock	TransactionStatus = 3

	// Errors:
	TransactionStatusError 				TransactionStatus = -1
	TransactionStatusErrorRejected 		TransactionStatus = -2
	TransactionStatusErrorReplaced 		TransactionStatus = -3
	// Transaction was mined, but the SC call that was performed by this transaction failed.
	TransactionStatusErrorSmartContractCallFailed TransactionStatus = -4
)

// BlockHeader is a header of the Ethereum blockchain block
type BlockHeader struct {
	ID BlockHash		`json:"_id" bson:"_id"`
	Height uint64		`json:"height" bson:"height"`
	Parent BlockHash 	`json:"parent_id" bson:"parent_id"`
}

// Block is an Ethereum blockchain block
type Block struct {
	BlockHeader					`json:",inline" bson:",inline"`
	Transactions []Transaction	`json:"transactions" bson:"transactions"`
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

const amountBSONStringBase = 10

func (a *Amount) SetBSON(raw bson.Raw) error {
	var amountString string
	err := raw.Unmarshal(&amountString)
	if err != nil {
		return errors.Wrap(err, "Faield to parse amount from BSON")
	}

	amount, err := NewAmountFromString(amountString, amountBSONStringBase)
	if err != nil {
		return err
	}
	*a = *amount

	return nil
}

func (a *Amount) GetBSON() (interface{}, error) {
	return a.Text(amountBSONStringBase), nil
}