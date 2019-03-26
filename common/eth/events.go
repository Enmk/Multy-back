package eth

type TransactionStatusEvent struct {
	ID TransactionHash
	Status TransactionStatus
	BlockHash BlockHash
}