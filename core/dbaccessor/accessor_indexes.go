package dbaccessor

import (
	"encoding/binary"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

func ReadTxSavedBlockHeightAndHash(db DatabaseReader) (uint64, common.Hash, error) {
	data, err := db.Get(txSavedBlockKey())
	if err != nil {
		return 0, common.Hash{}, err
	}

	if len(data) != 40 {
		return 0, common.Hash{}, ErrDataLength
	}
	height := binary.BigEndian.Uint64(data[0:8])
	var hash common.Hash
	copy(hash[:], data[8:40])
	return height, hash, nil
}

func WriteTxSavedBlockHeightAndHash(db DatabaseWriter, height uint64, hash common.Hash) {
	var data [40]byte
	binary.BigEndian.PutUint64(data[0:8], height)
	copy(data[8:40], hash[:])
	if err := db.Put(txSavedBlockKey(), data[:]); err != nil {
		log.Crit("Failed to store tx saved block reach height", "err", err)
	}
}

func ReadTxLookupEntry(db DatabaseReader, hash common.Hash) (TxLookupEntry, error) {
	data, err := ReadTxLookupEntryRLP(db, hash)
	if err != nil {
		return TxLookupEntry{}, err
	}
	var entry TxLookupEntry
	if err := rlp.DecodeBytes(data, &entry); err != nil {
		log.Error("Invalid transaction lookup entry RLP", "hash", hash, "err", err)
		return TxLookupEntry{}, err
	}
	return entry, nil
}

func ReadTxLookupEntryRLP(db DatabaseReader, hash common.Hash) (rlp.RawValue, error) {
	data, err := db.Get(txLookupKey(hash))
	return data, err
}

func WriteTxLookupEntries(db dbwrapper.Database, blockHeight uint64, blockFullHash common.Hash, executedTxs []*types.TxWithIndex) {
	log.Info("WriteTxLookupEntries", "blockHeight", blockHeight, "blockFullHash", blockFullHash, "executedTxsNum", len(executedTxs))

	if len(executedTxs) == 0 {
		return
	}

	batch := db.NewBatch()

	for _, executedTx := range executedTxs {
		entry := TxLookupEntry{
			BlockFullHash:  blockFullHash,
			TxPackageIndex: executedTx.TxPackageIndex,
			TxIndex:        executedTx.TxIndex,
		}
		data, err := rlp.EncodeToBytes(entry)
		if err != nil {
			log.Crit("Failed to encode transaction lookup entry", "err", err)
		}
		if err := batch.Put(txLookupKey(executedTx.Tx.Hash()), data); err != nil {
			log.Crit("Failed to store transaction lookup entry", "err", err)
		}
	}

	if err := batch.Write(); err != nil {
		log.Crit("Failed to write transaction lookup entry", "err", err)
	}
}
