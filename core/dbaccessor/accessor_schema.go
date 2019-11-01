package dbaccessor

import (
	"encoding/binary"
	"errors"

	"github.com/fractal-platform/fractal/common"
)

const (
	RoundStep = 10
)

var (
	ErrDataLength = errors.New("data length error")
)

// The fields below define the low level database schema prefixing.
var (
	// databaseVersionKey tracks the current database version.
	databaseVersionKey = []byte("DatabaseVersion")

	// chainConfigKey stores the config for current chain.
	chainConfigKey = []byte("ChainConfig")

	// headBlockKey tracks the latest know full block's hash.
	headBlockKey = []byte("LastBlock")

	// genesisBlockKey stores genesis block's hash.
	genesisBlockKey = []byte("GenesisBlock")

	// checkpoint
	lastCheckPointKey = []byte("LCP")
	checkPointPrefix  = []byte("CP")

	// Data item prefixes (use single byte to avoid mixing data types, avoid `i`, used for indexes).
	headerPrefix          = []byte("H")  // headerPrefix + hash -> block header
	bodyPrefix            = []byte("B")  // bodyPrefix + hash -> block body
	receivePathPrefix     = []byte("P")  // receivePathPrefix + hash -> block receive path
	blockChildPrefix      = []byte("C")  // blockChildPrefix + hash -> child hash list
	blockRoundHashPrefix  = []byte("R")  // blockRoundHashPrefix + round (uint64 big endian) -> round-hash list
	blockReceiptsPrefix   = []byte("r")  // blockReceiptsPrefix + hash -> block receipts
	blockBloomPrefix      = []byte("b")  // blockBloomPrefix + hash -> block bloom
	blockStateCheckPrefix = []byte("SC") // blockStateCheckPrefix + hash -> state checked flag
	heightBlockMapPrefix  = []byte("HBM")

	// height and hash of the highest block whose txs has been saved into db
	savedTxBlockPrefix = []byte("STB") // savedTxBlockPrefix -> saved tx block flag

	heightBlocksPrefix = []byte("NN")

	mainBranchHeadPrefix = []byte("MBH")

	bloomBitsPrefix                = []byte("BB") // bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian) -> bloom bits
	bloomSectionSavedFlagPrefix    = []byte("BSF")
	bloomFastSyncReachHeightPrefix = []byte("BFS")

	accHashPrefix = []byte("ACC")

	txLookupPrefix = []byte("l") // txLookupPrefix + hash -> transaction lookup metadata

	txPkgNoncePrefix = []byte("PN") // txPackageNoncePrefix + coinbase -> the current package nonce of this coinbase
	txPkgHashPrefix  = []byte("PH") // txPkgHashPrefix + hash -> the txPackage data

	pkgPoolHashPrefix = []byte("PPH")
)

// encodeBlockRound encodes a block round as big endian uint64
func encodeBlockRound(round uint64) []byte {
	enc := make([]byte, 8)
	binary.BigEndian.PutUint64(enc, round)
	return enc
}

// blockHeaderKey = headerPrefix + hash
func blockHeaderKey(hash common.Hash) []byte {
	return append(headerPrefix, hash.Bytes()...)
}

// blockBodyKey = bodyPrefix + hash
func blockBodyKey(hash common.Hash) []byte {
	return append(bodyPrefix, hash.Bytes()...)
}

// blockReceivePathKey = receivePathPrefix + hash
func blockReceivePathKey(hash common.Hash) []byte {
	return append(receivePathPrefix, hash.Bytes()...)
}

// blockChildKey = blockChildPrefix + hash
func blockChildKey(hash common.Hash) []byte {
	return append(blockChildPrefix, hash.Bytes()...)
}

// blockRoundHashKey = blockRoundHashPrefix + round
func blockRoundHashKey(round uint64) []byte {
	return append(blockRoundHashPrefix, encodeBlockRound(round)...)
}

// blockStateCheckKey = blockStateCheckPrefix + hash
func blockStateCheckKey(hash common.Hash) []byte {
	return append(blockStateCheckPrefix, hash.Bytes()...)
}

// TxLookupEntry is a positional metadata to help looking up the data content of
// a transaction or receipt given only its hash.
type TxLookupEntry struct {
	BlockFullHash  common.Hash
	TxPackageIndex uint32
	TxIndex        uint32
}

type TxLookupList []TxLookupEntry

// txLookupKey = txLookupPrefix + hash
func txLookupKey(hash common.Hash) []byte {
	return append(txLookupPrefix, hash.Bytes()...)
}

func txSavedBlockKey() []byte {
	return savedTxBlockPrefix
}

// blockReceiptsKey = blockReceiptsPrefix + hash
func blockReceiptsKey(hash common.Hash) []byte {
	return append(blockReceiptsPrefix, hash.Bytes()...)
}

// blockBloomKey = blockBloomPrefix + hash
func blockBloomKey(hash common.Hash) []byte {
	return append(blockBloomPrefix, hash.Bytes()...)
}

// bloomBitsKey = bloomBitsPrefix + bit (uint16 big endian) + section (uint64 big endian)
func bloomBitsKey(bit uint, section uint64) []byte {
	key := append(bloomBitsPrefix, make([]byte, 10)...)

	binary.BigEndian.PutUint16(key[2:], uint16(bit))
	binary.BigEndian.PutUint64(key[4:], section)

	return key
}

func bloomSectionSavedFlagKey(section uint64) []byte {
	key := append(bloomSectionSavedFlagPrefix, make([]byte, 8)...)
	binary.BigEndian.PutUint64(key[3:], section)
	return key
}

func bloomFastSyncReachHeightMapKey() []byte {
	return bloomFastSyncReachHeightPrefix
}

// accHashKey = accHashPrefix + blockFullHash
func accHashKey(hash common.Hash) []byte {
	return append(accHashPrefix, hash.Bytes()...)
}

func heightBlockMapKey(height uint64) []byte {
	key := append(heightBlockMapPrefix, make([]byte, 8)...)
	binary.BigEndian.PutUint64(key[3:], height)
	return key
}

func heightBlocksKey(height uint64) []byte {
	key := append(heightBlocksPrefix, make([]byte, 8)...)
	binary.BigEndian.PutUint64(key[2:], height)
	return key
}

func mainBranchHeadKey() []byte {
	return mainBranchHeadPrefix
}

func txPackageNonceKey(coinbase common.Address) []byte {
	return append(txPkgNoncePrefix, coinbase.Bytes()...)
}

func txPackageHashKey(hash common.Hash) []byte {
	return append(txPkgHashPrefix, hash.Bytes()...)
}

func checkPointKey(index uint64) []byte {
	key := append(checkPointPrefix, make([]byte, 8)...)
	binary.BigEndian.PutUint64(key[2:], index)
	return key
}

// DatabaseReader wraps the Has and Get method of a backing data store.
type DatabaseReader interface {
	Has(key []byte) (bool, error)
	Get(key []byte) ([]byte, error)
}

// DatabaseWriter wraps the Put method of a backing data store.
type DatabaseWriter interface {
	Put(key []byte, value []byte) error
}

// DatabaseDeleter wraps the Delete method of a backing data store.
type DatabaseDeleter interface {
	Delete(key []byte) error
}
