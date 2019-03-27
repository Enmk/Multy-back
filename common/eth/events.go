package eth

type TransactionStatusEvent struct {
	TransactionHash TransactionHash
	Status          TransactionStatus
	BlockHash       BlockHash
}