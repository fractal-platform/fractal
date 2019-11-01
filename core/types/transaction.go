package types

import (
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/common/math"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rlp"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

type Transaction struct {
	data txdata
	// caches
	hash        atomic.Value
	size        atomic.Value
	from        atomic.Value
	packingHash atomic.Value
}

type txdata struct {
	AccountNonce uint64          `json:"nonce"     gencodec:"required"`
	Price        *big.Int        `json:"gasPrice"  gencodec:"required"`
	GasLimit     uint64          `json:"gas"       gencodec:"required"`
	Recipient    *common.Address `json:"to"        rlp:"nil"` // nil means contract creation
	Amount       *big.Int        `json:"value"     gencodec:"required"`
	Payload      []byte          `json:"input"     gencodec:"required"`
	Broadcast    bool            `json:"broadcast" gencodec:"required"`

	// Signature values
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}

func NewTransaction(nonce uint64, to common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) *Transaction {
	return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data, broadcast)
}

func NewContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) *Transaction {
	return newTransaction(nonce, nil, amount, gasLimit, gasPrice, data, broadcast)
}

func newTransaction(nonce uint64, to *common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, broadcast bool) *Transaction {
	if len(data) > 0 {
		data = common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Broadcast:    broadcast,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &Transaction{data: d}
}

// ChainId returns which chain id this transaction was signed for (if at all)
func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}

// EncodeRLP implements rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP implements rlp.Decoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}

	return err
}

// MarshalJSON encodes the web3 RPC transaction format.
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}

// UnmarshalJSON decodes the web3 RPC transaction format.
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	*tx = Transaction{data: dec}
	return nil
}

func (tx *Transaction) Data() []byte       { return common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64        { return tx.data.GasLimit }
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.data.Price) }
func (tx *Transaction) Value() *big.Int    { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64      { return tx.data.AccountNonce }
func (tx *Transaction) CheckNonce() bool   { return true }
func (tx *Transaction) Broadcast() bool    { return tx.data.Broadcast }

// To returns the recipient address of the transaction.
// It returns nil if the transaction is a contract creation.
func (tx *Transaction) To() *common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}

// Hash hashes the RLP encoding of tx.
// It uniquely identifies the transaction.
func (tx *Transaction) Hash() common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := common.RlpHash(tx)
	tx.hash.Store(v)
	return v
}

func (tx *Transaction) PackingHash(signer Signer) common.Hash {
	if packingHash := tx.packingHash.Load(); packingHash != nil {
		return packingHash.(common.Hash)
	}
	from, _ := Sender(signer, tx) // has checked, ignore error
	v := common.RlpHash([]interface{}{
		from,
		tx.data.AccountNonce,
	})
	tx.packingHash.Store(v)
	return v
}

func (tx *Transaction) PackingHashUint64(signer Signer) uint64 {
	v := tx.PackingHash(signer)
	return v.Big().Uint64()
}

func (tx *Transaction) MatchPacker(packerGroupSize uint64, packerIndex uint32, signer Signer) bool {
	txPackingHashUint64 := tx.PackingHashUint64(signer)
	return txPackingHashUint64%packerGroupSize == uint64(packerIndex)%packerGroupSize
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previsouly cached value.
func (tx *Transaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

// AsMessage returns the transaction as a types.Message.
//
// AsMessage requires a signer to derive the sender.
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}

// WithSignature returns a new transaction with the given signature.
// This signature is generated by Sign method in the crypto.PrivateKey interface.
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}

func GasFee(gas uint64, gasPrice *big.Int) *big.Int {
	return new(big.Int).Quo(new(big.Int).Mul(new(big.Int).SetUint64(gas), gasPrice), params.TransactionFeeCoefficient)
}

// Cost returns amount + gasprice * gaslimit / 10.
func (tx *Transaction) Cost() *big.Int {
	total := GasFee(tx.data.GasLimit, tx.data.Price)
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}

func (tx *Transaction) SetFrom(signer Signer, addr common.Address) {
	tx.from.Store(sigCache{signer: signer, from: addr})
}

// Transactions is a Transaction slice type for basic sorting.
type Transactions []*Transaction

// Len returns the length of s.
func (s Transactions) Len() int { return len(s) }

// Swap swaps the i'th and the j'th element in s.
func (s Transactions) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (s Transactions) GetRlp(i int) []byte {
	enc, _ := rlp.EncodeToBytes(s[i])
	return enc
}

// TxDifference returns a new set which is the difference between a and b.
func TxDifference(a, b Transactions) Transactions {
	keep := make(Transactions, 0, len(a))

	remove := make(map[common.Hash]struct{})
	for _, tx := range b {
		remove[tx.Hash()] = struct{}{}
	}

	for _, tx := range a {
		if _, ok := remove[tx.Hash()]; !ok {
			keep = append(keep, tx)
		}
	}

	return keep
}

// TxByNonce implements the sort interface to allow sorting a list of transactions
// by their nonces. This is usually only useful for sorting transactions from a
// single account, otherwise a nonce comparison doesn't make much sense.
type TxByNonce Transactions

func (s TxByNonce) Len() int           { return len(s) }
func (s TxByNonce) Less(i, j int) bool { return s[i].data.AccountNonce < s[j].data.AccountNonce }
func (s TxByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// TxByPrice implements both the sort and the heap interface, making it useful
// for all at once sorting as well as individually adding and removing elements.
type TxByPrice Transactions

func (s TxByPrice) Len() int           { return len(s) }
func (s TxByPrice) Less(i, j int) bool { return s[i].data.Price.Cmp(s[j].data.Price) > 0 } // We want Pop to give us the highest gas price one, not lowest, priority so we use greater than here.
func (s TxByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *TxByPrice) Push(x interface{}) {
	*s = append(*s, x.(*Transaction))
}

func (s *TxByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

// Message is a fully derived transaction and implements types.Message
type Message struct {
	to         *common.Address
	from       common.Address
	nonce      uint64
	amount     *big.Int
	gasLimit   uint64
	gasPrice   *big.Int
	data       []byte
	checkNonce bool
}

func NewMessage(from common.Address, to *common.Address, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte, checkNonce bool) Message {
	return Message{
		from:       from,
		to:         to,
		nonce:      nonce,
		amount:     amount,
		gasLimit:   gasLimit,
		gasPrice:   gasPrice,
		data:       data,
		checkNonce: checkNonce,
	}
}

func (m Message) From() common.Address { return m.from }
func (m Message) To() *common.Address  { return m.to }
func (m Message) GasPrice() *big.Int   { return m.gasPrice }
func (m Message) Value() *big.Int      { return m.amount }
func (m Message) Gas() uint64          { return m.gasLimit }
func (m Message) Nonce() uint64        { return m.nonce }
func (m Message) Data() []byte         { return m.data }
func (m Message) CheckNonce() bool     { return m.checkNonce }

type TxWithIndex struct {
	Tx             *Transaction
	TxPackageIndex uint32 // index in txPackage
	TxIndex        uint32 // index in block
}

// If a transaction not in txPackage, the TxPackageIndex is NotInPackage.
const NotInPackage uint32 = math.MaxUint32
