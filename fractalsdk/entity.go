package fractalsdk

import (
	"math/big"
)

type Block struct {
	FullHash   string
	SimpleHash string

	ParentHash     string
	Round          uint64
	Sig            string
	Coinbase       string
	Difficulty     *big.Int
	Height         uint64
	Amount         uint64
	GasLimit       uint64
	GasUsed        uint64
	StateHash      string
	TxHash         string
	ReceiptHash    string
	ParentFullHash string
	Confirms       []string
	FullSig        string
	MinedTime      uint64
	HopCount       uint64

	Transactions    []*Transaction
	TxPackageHashes []string
}

type TxPackage struct {
	Packer        string
	PackNonce     uint64
	Transactions  []*Transaction
	BlockFullHash string
	R             *big.Int
	S             *big.Int
	V             *big.Int
	Hash          string
	GenTime       uint64
}

type Transaction struct {
	AccountNonce uint64
	Price        *big.Int
	GasLimit     uint64
	Recipient    string
	Amount       *big.Int
	Payload      []byte
	Broadcast    bool
	V            *big.Int
	R            *big.Int
	S            *big.Int
	Hash         string
}

type Log struct {
	Address     string
	Topics      []string
	Data        []byte
	BlockNumber uint64
	TxHash      string
	PkgIndex    uint32
	TxIndex     uint32
	Index       uint32
}

type Receipt struct {
	PostState         []byte
	Status            uint64
	CumulativeGasUsed uint64
	Bloom             *[4096]byte
	Logs              []*Log
	TxHash            string
	ContractAddress   string
	GasUsed           uint64
}

type TransactionDetails struct {
	From      string
	Hash      string
	Nonce     uint64
	To        string
	Value     *big.Int
	Price     *big.Int
	GasLimit  uint64
	Payload   []byte
	Broadcast bool
	V         *big.Int
	R         *big.Int
	S         *big.Int
	BlockHash string
	Receipt   *Receipt
}

type CallResult struct {
	Logs    []*Log
	GasUsed uint64
}

type ExecResult struct {
	TxDetails *TransactionDetails
	Err       error
}

type ExecTask struct {
	// input
	To                string
	Amount            *big.Int
	GasLimit          uint64
	GasPrice          *big.Int
	Data              []byte
	Broadcast         bool
	SecondWaitForExec int

	// output
	TxDetails *TransactionDetails
	Err       error
}
