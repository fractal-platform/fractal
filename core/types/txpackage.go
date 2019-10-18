package types

import (
	"errors"
	"fmt"
	"io"
	"math/big"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/rlp"
)

//go:generate gencodec -type pkgData -field-override pkgDataMarshaling -out gen_txpackage_json.go

var (
	ErrInvalidPkgSig = errors.New("invalid transaction package v, r, s values")
)

type PkgReceivePath byte

const (
	PkgReceivePathBegin PkgReceivePath = iota
	PkgReceivePathBroadcast
	PkgReceivePathSync
	PkgReceivePathFuture
	PkgReceivePathEnd
)

func (p PkgReceivePath) String() string {
	if p <= PkgReceivePathBegin || p >= PkgReceivePathEnd {
		return "Unknown"
	}

	list := [...]string{
		"Broadcast",
		"Sync",
		"Future",
		"PeerSync"}
	return list[p-1]
}

type TxPackage struct {
	data pkgData
	// caches:
	hash         atomic.Value
	ReceivedAt   time.Time
	ReceivedFrom interface{}
	ReceivedPath PkgReceivePath
}

type pkgData struct {
	Packer        common.Address `json:"packer"`
	PackNonce     uint64         `json:"nonce"`
	Transactions  []*Transaction `json:"transactions"`
	BlockFullHash common.Hash    `json:"blockFullHash"`

	R *big.Int `json:"r"`
	S *big.Int `json:"s"`
	V *big.Int `json:"v"`
	// only for json serial:
	Hash *common.Hash `json:"hash" rlp:"-"`

	GenTime  uint64 `json:"genTime"`
	HopCount uint64 `json:"hopCount"`
}

type pkgDataMarshaling struct {
	PackNonce hexutil.Uint64
	V         *hexutil.Big
	R         *hexutil.Big
	S         *hexutil.Big
}

func NewTxPackage(packer common.Address, nonce uint64, txs []*Transaction, blockFullHash common.Hash, genTime uint64) *TxPackage {
	if txs == nil {
		txs = make([]*Transaction, 0)
	}
	return &TxPackage{
		data: pkgData{
			Packer:        packer,
			PackNonce:     nonce,
			Transactions:  txs,
			BlockFullHash: blockFullHash,
			GenTime:       genTime,
		},
	}
}

func (pkg *TxPackage) MarshalJSON() ([]byte, error) {
	hash := pkg.Hash()
	data := pkg.data
	data.Hash = &hash
	return data.MarshalJSON()
}

func (pkg *TxPackage) UnmarshalJSON(input []byte) error {
	var data pkgData
	if err := data.UnmarshalJSON(input); err != nil {
		return err
	}
	*pkg = TxPackage{data: data}
	return nil
}

func (pkg *TxPackage) DecodeRLP(s *rlp.Stream) error {
	if err := s.Decode(&pkg.data); err != nil {
		return err
	}
	return nil
}

func (pkg *TxPackage) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &pkg.data)
}

// Hash encode the pkgData with rlp and calc the hash value.
// This is the unique id of a txPackage.
func (pkg *TxPackage) Hash() common.Hash {
	if hash := pkg.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	h := rlpHash([]interface{}{
		pkg.data.Packer,
		pkg.data.PackNonce,
		pkg.data.Transactions,
		pkg.data.BlockFullHash,
	})
	pkg.hash.Store(h)
	return h
}

func (pkg *TxPackage) Packer() common.Address       { return pkg.data.Packer }
func (pkg *TxPackage) Nonce() uint64                { return pkg.data.PackNonce }
func (pkg *TxPackage) Transactions() []*Transaction { return pkg.data.Transactions }
func (pkg *TxPackage) BlockFullHash() common.Hash   { return pkg.data.BlockFullHash }
func (pkg *TxPackage) GasPrice() *big.Int           { return common.Big0 } // to implement pool.Element interface

func (pkg *TxPackage) AddTransactions(txs []*Transaction) []*Transaction {
	if txs == nil {
		return pkg.data.Transactions
	}
	pkg.data.Transactions = append(pkg.data.Transactions, txs...)
	return pkg.data.Transactions
}

func (pkg *TxPackage) Signature() (r *big.Int, s *big.Int, v *big.Int) {
	r = pkg.data.R
	s = pkg.data.S
	v = pkg.data.V
	return r, s, v
}

func (pkg *TxPackage) SetSignature(R, S, V *big.Int) {
	pkg.data.R, pkg.data.S, pkg.data.V = R, S, V
}

func (pkg *TxPackage) Fork() *TxPackage {
	return &TxPackage{data: pkg.data}
}

func (pkg *TxPackage) GenTime() uint64 {
	return pkg.data.GenTime
}

func (pkg *TxPackage) IncreaseHopCount() {
	pkg.data.HopCount++
}

func (pkg *TxPackage) HopCount() uint64 {
	return pkg.data.HopCount
}

// type for txpkg array
type TxPackages []*TxPackage

//
func (pkgs *TxPackages) Has(hash common.Hash) bool {
	if pkgs == nil {
		return false
	}
	for _, pkg := range *pkgs {
		if pkg.Hash() == hash {
			return true
		}
	}
	return false
}

// remove txpkg from array
func (pkgs *TxPackages) Remove(hash common.Hash) {
	for i, pkg := range *pkgs {
		if pkg.Hash() == hash {
			*pkgs = append((*pkgs)[:i], (*pkgs)[i+1:]...)
			return
		}
	}
}

//
func (pkgs *TxPackages) Copy() TxPackages {
	ret := make(TxPackages, 0)
	ret = append(ret, *pkgs...)
	return ret
}

type PkgSigner interface {
	// Sign the txPackage with the given key, and return a new signed package.
	Sign(pkg *TxPackage, key crypto.PrivateKey) (*TxPackage, error)

	// VerifySignature verify the given package's signature
	RecoverPubKey(pkg *TxPackage) ([]byte, error)

	// Hash return the package hash that should be signed.
	Hash(pkg *TxPackage) common.Hash

	// SigToRSV transfer the signature into [r||s||v] format.
	SigToRSV(sig []byte) (r, s, v *big.Int, err error)

	// Equals return if the given signer == this one.
	Equals(signer PkgSigner) bool
}

func MakePkgSigner(fakeMode bool) PkgSigner {
	if !fakeMode {
		return NewDefaultSigner()
	}
	return nil
}

type defaultSigner struct{}

const magicNum = byte(27)

func NewDefaultSigner() PkgSigner {
	return defaultSigner{}
}

func (ds defaultSigner) SigToRSV(sig []byte) (r, s, v *big.Int, err error) {
	if len(sig) != 65 {
		panic(fmt.Sprintf("wrong size for signature: got %d, want 65", len(sig)))
	}
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64] + magicNum})
	return r, s, v, nil
}

func (ds defaultSigner) Sign(pkg *TxPackage, key crypto.PrivateKey) (*TxPackage, error) {
	sign, err := key.Sign(ds.Hash(pkg).Bytes())
	if err != nil {
		return nil, err
	}
	r, s, v, err := ds.SigToRSV(sign)
	if err != nil {
		return nil, err
	}
	cpy := pkg.Fork()
	cpy.SetSignature(r, s, v)
	return cpy, nil
}

func (ds defaultSigner) RecoverPubKey(pkg *TxPackage) ([]byte, error) {
	R, S, Vb := pkg.Signature()
	if Vb.BitLen() > 8 {
		return nil, ErrInvalidPkgSig
	}
	V := byte(Vb.Uint64() - 27)
	if !crypto.ValidateSignatureValues(V, R, S, true) {
		return nil, ErrInvalidPkgSig
	}
	// encode the snature in uncompressed format
	r, s := R.Bytes(), S.Bytes()
	sig := make([]byte, 65)
	copy(sig[32-len(r):32], r)
	copy(sig[64-len(s):64], s)
	sig[64] = V
	// recover the public key from the snature
	hash := ds.Hash(pkg)
	pub, err := crypto.Ecrecover(hash[:], sig)
	if err != nil {
		return nil, err
	}
	if len(pub) == 0 || pub[0] != 4 {
		return nil, errors.New("invalid public key")
	}
	return pub, nil
}

func (defaultSigner) Hash(pkg *TxPackage) common.Hash {
	txs := pkg.Transactions()
	txHashs := make([]common.Hash, len(txs))
	for i, t := range txs {
		txHashs[i] = t.Hash()
	}
	return rlpHash([]interface{}{
		pkg.data.Packer,
		pkg.data.PackNonce,
		pkg.data.BlockFullHash,
		txHashs,
	})
}

func (defaultSigner) Equals(signer PkgSigner) bool {
	_, ok := signer.(defaultSigner)
	return ok
}
