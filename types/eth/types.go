package eth

import "math/big"

// Hash of the Block, often used as ID
type BlockHash string

// Hash of the Transaction, often used as ID
type TransactionHash string

// Blockchain address of the sender/recepient 
type Address string

type TransactionPayload []byte

type TransactionNonce uint64

// surprisingly hard to work with type aliases
// type Amount big.Int

// Transaction is an Ethereun blockchain transaction
type Transaction struct {
	ID TransactionHash			`json:"_id" bson:"_id"`
	Sender Address				`json:"sender" bson:"sender"`
	Receiver Address			`json:"received" bson:"received"`
	Payload TransactionPayload	`json:"payload" bson:"payload"`
	Amount *big.Int				`json:"amount" bson:"amount"`
	Nonce TransactionNonce		`json:"nonce" bson:"nonce"`
}

type TransactionWithStatus struct {
	Transaction
	Status TransactionStatus	`json:"status" bson:"status"`
}

// Positive values mean non-error statuses, Negative mean different error conditions
type TransactionStatus int

const (
	TransactionStatusInMempool = 1
	TransactionStatusInBlock = 2
	TransactionStatusInImmutableBlock = 3

	TransactionStatusError = -1 // general error code
	TransactionStatusErrorRejected = -2
	TransactionStatusErrorReplaced = -3
	// Transaction was mined, but the SC call that was performed by this transaction failed.
	TransactionStatusErrorSmartContractCallFailed = -3
)

// BlockHeader is a header of the Ethereum blockchain block
type BlockHeader struct {
	ID BlockHash		`json:"_id" bson:"_id"`
	Height uint64		`json:"height" bson:"height"`
	Parent BlockHash 	`json:"parent_id" bson:"parent_id"`
}

// Block is an Ethereum blockchain block
type Block struct {
	BlockHeader				`json:"header" bson:"header"`
	Transactions []Transaction	`json:"transactions" bson:"transactions"`
}