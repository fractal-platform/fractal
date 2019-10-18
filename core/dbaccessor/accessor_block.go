package dbaccessor

import (
	"bytes"
	"encoding/binary"
	"errors"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/bitutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// ReadHeadBlockHash retrieves the hash of the current canonical head block.
func ReadHeadBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(headBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}

	var hash common.Hash
	err := rlp.DecodeBytes(data, &hash)
	if err != nil {
		log.Error("DecodeBytes error: " + err.Error())
	}
	log.Debug("ReadHeadBlockHash: db get data", "hash", hash.String())
	return hash
}

// WriteHeadBlockHash stores the head block's hash.
func WriteHeadBlockHash(db DatabaseWriter, hash common.Hash) {
	log.Debug("WriteHeadBlockHash", "hash", hash.String())
	hashBytes, _ := rlp.EncodeToBytes(hash)
	if err := db.Put(headBlockKey, hashBytes); err != nil {
		log.Crit("Failed to store last block's hash", "err", err)
	}
}

// ReadGenesisBlockHash retrieves the hash of the genesis block.
func ReadGenesisBlockHash(db DatabaseReader) common.Hash {
	data, _ := db.Get(genesisBlockKey)
	if len(data) == 0 {
		return common.Hash{}
	}

	var hash = common.Hash{}
	err := rlp.DecodeBytes(data, &hash)
	if err != nil {
		log.Error("DecodeBytes error: " + err.Error())
	}
	log.Debug("ReadGenesisBlockHash: db get data", "hash", hash)
	return hash
}

// WriteGenesisBlockHash stores the genesis block's hash.
func WriteGenesisBlockHash(db DatabaseWriter, hash common.Hash) {
	log.Debug("WriteGenesisBlockHash", "hash", hash)
	hashBytes, _ := rlp.EncodeToBytes(hash)
	if err := db.Put(genesisBlockKey, hashBytes); err != nil {
		log.Crit("Failed to store genesis block's hash", "err", err)
	}
}

// ReadBlockHeaderRLP retrieves a block header in its raw RLP database encoding.
func ReadBlockHeaderRLP(db DatabaseReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(blockHeaderKey(hash))
	return data
}

// ReadBlockBodyRLP retrieves a block body in its raw RLP database encoding.
func ReadBlockBodyRLP(db DatabaseReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(blockBodyKey(hash))
	return data
}

// HasBlock verifies the existence of a block header corresponding to the hash.
func HasBlock(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(blockHeaderKey(hash)); !has || err != nil {
		return false
	}
	return true
}

// ReadBlockHeader retrieves the block corresponding to the hash.
func ReadBlockHeader(db DatabaseReader, hash common.Hash) *types.BlockHeader {
	data := ReadBlockHeaderRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	var header types.BlockHeader
	err := rlp.DecodeBytes(data, &header)
	if err != nil {
		log.Error("Invalid block header RLP", "hash", hash, "err", err)
		return nil
	}
	return &header
}

// ReadBlockBody retrieves the block corresponding to the hash.
func ReadBlockBody(db DatabaseReader, hash common.Hash) *types.BlockBody {
	data := ReadBlockBodyRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	var body types.BlockBody
	err := rlp.DecodeBytes(data, &body)
	if err != nil {
		log.Error("Invalid block body RLP", "hash", hash, "err", err)
		return nil
	}
	return &body
}

// ReadBlockReceivePath retrieves the block corresponding to the hash.
func ReadBlockReceivePath(db DatabaseReader, hash common.Hash) (types.BlockReceivePathEnum, error) {
	data, err := db.Get(blockReceivePathKey(hash))
	if err != nil || len(data) != 1 {
		log.Error("ReadBlockReceivePath error", "hash", hash, "err", err)
		return 0, err
	}

	return types.BlockReceivePathEnum(data[0]), nil
}

// WriteBlock stores a block header into the database
func WriteBlock(db DatabaseWriter, block *types.Block) {
	// Write the hash -> height mapping
	var (
		hash   = block.FullHash()
		round  = block.Header.Round
		height = block.Header.Height
	)
	log.Debug("WriteBlock", "round", round, "height", height, "hash", hash)

	// Write the encoded body, write body first, then header
	bodyBytes, err := rlp.EncodeToBytes(block.Body)
	if err != nil {
		log.Crit("Failed to RLP encode block body", "err", err)
	}
	bodyKey := blockBodyKey(hash)
	if err := db.Put(bodyKey, bodyBytes); err != nil {
		log.Crit("Failed to store block body", "err", err)
	}

	// Write the encoded header
	headerBytes, err := rlp.EncodeToBytes(block.Header)
	if err != nil {
		log.Crit("Failed to RLP encode block header", "err", err)
	}
	headerKey := blockHeaderKey(hash)
	if err := db.Put(headerKey, headerBytes); err != nil {
		log.Crit("Failed to store block header", "err", err)
	}

	// Write the receieve path
	if err := db.Put(blockReceivePathKey(hash), []byte{byte(block.ReceivedPath)}); err != nil {
		log.Crit("Failed to store block received path", "err", err)
	}
}

// DeleteBlock removes all block data associated with a hash.
func DeleteBlock(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(blockHeaderKey(hash)); err != nil {
		log.Crit("Failed to delete header", "err", err)
	}

	if err := db.Delete(blockBodyKey(hash)); err != nil {
		log.Crit("Failed to delete body", "err", err)
	}

	if err := db.Delete(blockReceivePathKey(hash)); err != nil {
		log.Crit("Failed to delete received path", "err", err)
	}
}

// ReadBlockChilds returns the block childs assigned to a hash.
func ReadBlockChilds(db DatabaseReader, hash common.Hash) []common.Hash {
	data, _ := db.Get(blockChildKey(hash))
	if len(data) == 0 {
		return []common.Hash{}
	}

	var childs = []common.Hash{}
	err := rlp.DecodeBytes(data, &childs)
	if err != nil {
		log.Error("DecodeBytes error: " + err.Error())
	}
	return childs
}

// WriteBlockChilds returns the block childs assigned to a hash.
func WriteBlockChilds(db DatabaseWriter, hash common.Hash, childs []common.Hash) {
	log.Debug("WriteBlockChilds: ", "hash", hash, "len(childs)", len(childs))
	hashBytes, _ := rlp.EncodeToBytes(childs)
	if err := db.Put(blockChildKey(hash), hashBytes); err != nil {
		log.Crit("Failed to store hash to height mapping", "err", err)
	}
}

// DeleteBlockChilds removes the height to hash mapping.
func DeleteBlockChilds(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(blockChildKey(hash)); err != nil {
		log.Crit("Failed to delete hash to height mapping", "err", err)
	}
}

// Return blocks of reorg chain, include common ancestor
// (the first element of the result slice is the common ancestor, and the last element of the result slice is the new header)
func FindReorgChain(db DatabaseReader, old, new *types.BlockHeader) []*types.BlockHeader {
	var reorg []*types.BlockHeader
	if old.Height > new.Height {
		return nil
	}
	reorg = append(reorg, new)
	for an := old.Height; an < new.Height; {
		new = ReadBlockHeader(db, new.ParentFullHash)
		if new == nil {
			return nil
		}
		reorg = append(reorg, new)
	}
	for old.FullHash() != new.FullHash() {
		old = ReadBlockHeader(db, old.ParentFullHash)
		if old == nil {
			return nil
		}
		new = ReadBlockHeader(db, new.ParentFullHash)
		if new == nil {
			return nil
		}
		reorg = append(reorg, new)
	}

	// reverse
	for i, j := 0, len(reorg)-1; i < j; i, j = i+1, j-1 {
		reorg[i], reorg[j] = reorg[j], reorg[i]
	}
	return reorg
}

// ReadHashListByBlockRange retrieves the hash assigned to a block range (b1, b2]
func ReadHashListByBlockRange(db DatabaseReader, b1 *types.Block, b2 *types.Block) []*types.BlockRoundHash {
	log.Debug("ReadHashListByBlockRange: ",
		"round1", b1.Header.Round, "hash1", b1.SimpleHash(), "round2", b2.Header.Round, "hash2", b2.SimpleHash())
	var result = []*types.BlockRoundHash{}
	currentRound := b1.Header.Round - b1.Header.Round%RoundStep
	for {
		exist, _ := db.Has(blockRoundHashKey(currentRound))
		if exist {
			data, _ := db.Get(blockRoundHashKey(currentRound))
			if len(data) != 0 {
				var list = []*types.BlockRoundHash{}
				err := rlp.DecodeBytes(data, &list)
				if err != nil {
					log.Error("ReadHashListByBlockRange: DecodeBytes error: " + err.Error())
					return []*types.BlockRoundHash{}
				}
				for _, item := range list {
					if item.Round > b1.Header.Round && item.Round < b2.Header.Round {
						result = append(result, item)
					} else if item.Round == b1.Header.Round && bytes.Compare(item.SimpleHash.Bytes(), b1.SimpleHash().Bytes()) > 0 {
						result = append(result, item)
					} else if item.Round == b2.Header.Round && bytes.Compare(item.SimpleHash.Bytes(), b2.SimpleHash().Bytes()) <= 0 {
						result = append(result, item)
					}
				}
			}
		}

		//loop for next
		currentRound += RoundStep
		if currentRound > b2.Header.Round {
			break
		}
	}
	log.Debug("ReadHashListByBlockRange: db get data: ", "len", len(result))
	return result
}

// ReadHashListByBlockBackward retrieves the hash older than the block b
func ReadHashListByBlockBackward(db DatabaseReader, b *types.Block, num uint64) []*types.BlockRoundHash {
	log.Debug("ReadHashListByBlockBackward: ", "round", b.Header.Round, "hash", b.SimpleHash(), "num", num)
	var result = []*types.BlockRoundHash{}
	genesisHash := ReadGenesisBlockHash(db)
	currentRound := b.Header.Round - b.Header.Round%RoundStep
	for {
		exist, _ := db.Has(blockRoundHashKey(currentRound))
		if exist {
			data, _ := db.Get(blockRoundHashKey(currentRound))
			if len(data) != 0 {
				var list = []*types.BlockRoundHash{}
				err := rlp.DecodeBytes(data, &list)
				if err != nil {
					log.Error("ReadHashListByBlockBackward: DecodeBytes error: " + err.Error())
					return []*types.BlockRoundHash{}
				}
				for _, item := range list {
					if item.Round < b.Header.Round {
						result = append(result, item)
					} else if item.Round == b.Header.Round && bytes.Compare(item.SimpleHash.Bytes(), b.SimpleHash().Bytes()) < 0 {
						result = append(result, item)
					}
					if uint64(len(result)) >= num {
						return result
					}
				}

				if len(list) > 0 && list[0].FullHash == genesisHash {
					return result
				}
			}
		}

		//loop for next
		currentRound -= RoundStep
	}
}

// ReadHashListByRoundRange retrieves the hash assigned to a round range (r1, r2]
func ReadHashListByRoundRange(db DatabaseReader, r1 uint64, r2 uint64) []*types.BlockRoundHash {
	log.Debug("ReadHashListByRoundRange: ", "round1", r1, "round2", r2)
	var result = []*types.BlockRoundHash{}
	currentRound := r1 - r1%RoundStep
	for {
		exist, _ := db.Has(blockRoundHashKey(currentRound))
		if exist {
			data, _ := db.Get(blockRoundHashKey(currentRound))
			if len(data) != 0 {
				var list = []*types.BlockRoundHash{}
				err := rlp.DecodeBytes(data, &list)
				if err != nil {
					log.Error("ReadHashListByRoundRange: DecodeBytes error: " + err.Error())
					return []*types.BlockRoundHash{}
				}
				for _, item := range list {
					if item.Round > r1 && item.Round <= r2 {
						result = append(result, item)
					}
				}
			}
		}

		//loop for next
		currentRound += RoundStep
		if currentRound > r2 {
			break
		}
	}
	log.Debug("ReadHashListByRoundRange: db get data: ", "len", len(result))
	return result
}

// ReadHashListByRound retrieves the hash assigned to a round step range(related to param round)
func ReadHashListByRound(db DatabaseReader, round uint64) types.BlockRoundHashes {
	log.Debug("ReadHashListByRound: ", "round", round)
	var result = []*types.BlockRoundHash{}
	currentRound := round - round%RoundStep
	exist, _ := db.Has(blockRoundHashKey(currentRound))
	if exist {
		data, _ := db.Get(blockRoundHashKey(currentRound))
		if len(data) != 0 {
			err := rlp.DecodeBytes(data, &result)
			if err != nil {
				log.Error("ReadHashListByRound: DecodeBytes error: " + err.Error())
				return []*types.BlockRoundHash{}
			}
		}
	}
	log.Debug("ReadHashListByRound: db get data: ", "len", len(result))
	return result
}

func ReadBlockStateCheck(db DatabaseReader, hash common.Hash) types.BlockStateCheckedEnum {
	var result types.BlockStateCheckedEnum
	data, err := db.Get(blockStateCheckKey(hash))
	if err != nil || len(data) != 1 {
		//log.Error("ReadBlockStateCheck error", "hash", hash, "err", err)
		return result
	}
	result = types.BlockStateCheckedEnum(data[0])
	log.Info("ReadBlockStateCheck", "hash", hash, "result", result)
	return result
}

// WriteBlockStateCheck writes the state check enum into db.
func WriteBlockStateCheck(db DatabaseWriter, hash common.Hash, value types.BlockStateCheckedEnum) {
	if err := db.Put(blockStateCheckKey(hash), []byte{byte(value)}); err != nil {
		log.Crit("Failed to store block state check flag", "err", err)
	}

	log.Info("WriteBlockStateCheck", "hash", hash, "enum", value)
}

// WritePkgPoolHashList stores the pending pkgs in pkgPool.
func WritePkgPoolHashList(db DatabaseWriter, hashList []common.Hash) {
	log.Debug("WritePkgpoolHashList: ", "len", len(hashList))
	hashBytes, _ := rlp.EncodeToBytes(hashList)
	if err := db.Put(pkgPoolHashPrefix, hashBytes); err != nil {
		log.Crit("Failed to store pkgPool to hash mapping", "err", err)
	}
}

func ReadPkgPoolHashList(db DatabaseReader) ([]common.Hash, error) {
	data, err := db.Get(pkgPoolHashPrefix)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	var hashList []common.Hash
	err = rlp.DecodeBytes(data, &hashList)
	if err != nil {
		log.Crit("Failed to decode rlp bytes of pkgPoolList.", "err", err)
		return nil, err
	}
	return hashList, nil
}
func DeletePkgPoolHashList(db DatabaseDeleter) {
	log.Debug("deletePkgpoolHashList: ")
	if err := db.Delete(pkgPoolHashPrefix); err != nil {
		log.Crit("Failed to delete hash to round mapping", "err", err)
	}
}

// WriteHashList stores the hash assigned to a block round.
func WriteHashList(db DatabaseWriter, round uint64, hashList []*types.BlockRoundHash) {
	log.Debug("WriteHashList: ", "round", round, "len", len(hashList))
	currentRound := round - round%RoundStep
	hashBytes, _ := rlp.EncodeToBytes(hashList)
	if err := db.Put(blockRoundHashKey(currentRound), hashBytes); err != nil {
		log.Crit("Failed to store round to hash mapping", "err", err)
	}
}

// DeleteHashList delete the hash assigned to a block round.
func DeleteHashList(db DatabaseDeleter, round uint64) {
	log.Debug("WriteHashList: ", "round", round)
	currentRound := round - round%RoundStep
	if err := db.Delete(blockRoundHashKey(currentRound)); err != nil {
		log.Crit("Failed to delete hash to round mapping", "err", err)
	}
}

// ReadReceipts retrieves all the transaction receipts belonging to a block.
func ReadReceipts(db DatabaseReader, hash common.Hash) types.Receipts {
	// Retrieve the flattened receipt slice
	data := ReadReceiptsRLP(db, hash)
	if len(data) == 0 {
		return nil
	}
	// Convert the revceipts from their storage form to their internal representation
	storageReceipts := []*types.ReceiptForStorage{}
	if err := rlp.DecodeBytes(data, &storageReceipts); err != nil {
		log.Error("Invalid receipt array RLP", "hash", hash, "err", err)
		return nil
	}
	receipts := make(types.Receipts, len(storageReceipts))
	for i, receipt := range storageReceipts {
		receipts[i] = (*types.Receipt)(receipt)
	}
	return receipts
}

func ReadReceiptsRLP(db DatabaseReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(blockReceiptsKey(hash))
	return data
}

// WriteReceipts stores all the transaction receipts belonging to a block.
func WriteReceipts(db DatabaseWriter, hash common.Hash, receipts types.Receipts) {
	// Convert the receipts into their storage form and serialize them
	storageReceipts := make([]*types.ReceiptForStorage, len(receipts))
	for i, receipt := range receipts {
		storageReceipts[i] = (*types.ReceiptForStorage)(receipt)
	}
	bytes, err := rlp.EncodeToBytes(storageReceipts)
	if err != nil {
		log.Crit("Failed to encode block receipts", "err", err)
	}
	// Store the flattened receipt slice
	if err := db.Put(blockReceiptsKey(hash), bytes); err != nil {
		log.Crit("Failed to store block receipts", "err", err)
	}
}

// DeleteReceipts removes all receipt data associated with a block hash.
func DeleteReceipts(db DatabaseDeleter, hash common.Hash) {
	if err := db.Delete(blockReceiptsKey(hash)); err != nil {
		log.Crit("Failed to delete block receipts", "err", err)
	}
}

// ReadBloom retrieves the transaction bloom belonging to a block.
func ReadBloom(db DatabaseReader, hash common.Hash) (*types.Bloom, error) {
	// Retrieve the flattened receipt slice
	rawData := ReadBloomRLP(db, hash)
	bloomData, err := bitutil.DecompressBytes(rawData, types.BloomByteLength)
	if err != nil {
		return nil, err
	}

	if len(bloomData) != types.BloomByteLength {
		return nil, errors.New("Failed to retrieve block bloom")
	}
	return types.BytesToBloom(bloomData), nil
}

func ReadBloomRLP(db DatabaseReader, hash common.Hash) rlp.RawValue {
	data, _ := db.Get(blockBloomKey(hash))
	return data
}

// WriteReceipts stores the transaction bloom belonging to a block.
func WriteBloom(db DatabaseWriter, hash common.Hash, bloom *types.Bloom) {
	// Store the flattened receipt slice
	if err := db.Put(blockBloomKey(hash), bitutil.CompressBytes(bloom[:])); err != nil {
		log.Crit("Failed to store block bloom", "err", err)
	}
}

// ReadBloomBits retrieves the compressed bloom bit vector belonging to the given
// section and bit index from the.
func ReadBloomBits(db DatabaseReader, bit uint, section uint64) ([]byte, error) {
	return db.Get(bloomBitsKey(bit, section))
}

// WriteBloomBits stores the compressed bloom bits vector belonging to the given
// section and bit index.
func WriteBloomBits(db DatabaseWriter, bit uint, section uint64, bits []byte) {
	if err := db.Put(bloomBitsKey(bit, section), bits); err != nil {
		log.Crit("Failed to store bloom bits", "err", err)
	}
}

func DeleteBloomBits(db DatabaseDeleter, bit uint, section uint64) error {
	return db.Delete(bloomBitsKey(bit, section))
}

func ReadHeightBlockMap(db DatabaseReader, height uint64) (common.Hash, error) {
	data, err := db.Get(heightBlockMapKey(height))
	return common.BytesToHash(data), err
}

func WriteHeightBlockMap(db DatabaseWriter, height uint64, blockFullHash common.Hash) {
	log.Info("MainBranchRecord WriteHeightBlockMap", "height", height, "blockFullHash", blockFullHash)
	if err := db.Put(heightBlockMapKey(height), blockFullHash.Bytes()); err != nil {
		log.Crit("Failed to store height block map", "err", err)
	}
}

func ReadHeightBlocks(db DatabaseReader, height uint64) ([]common.Hash, error) {
	data, _ := db.Get(heightBlocksKey(height))
	if len(data) == 0 {
		return []common.Hash{}, nil
	}

	var hashes = []common.Hash{}
	err := rlp.DecodeBytes(data, &hashes)
	if err != nil {
		log.Error("DecodeBytes error: " + err.Error())
		return []common.Hash{}, err
	}
	return hashes, nil
}

func WriteHeightBlocks(db DatabaseWriter, height uint64, hashes []common.Hash) {
	hashBytes, _ := rlp.EncodeToBytes(hashes)
	if err := db.Put(heightBlocksKey(height), hashBytes); err != nil {
		log.Crit("Failed to store height to hashes mapping", "err", err)
	}
}

func ReadMainBranchHeadHeightAndHash(db DatabaseReader) (uint64, common.Hash, error) {
	data, err := db.Get(mainBranchHeadKey())
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

func WriteMainBranchHeadHeightAndHash(db DatabaseWriter, height uint64, hash common.Hash) {
	var data [40]byte
	binary.BigEndian.PutUint64(data[0:8], height)
	copy(data[8:40], hash[:])
	if err := db.Put(mainBranchHeadKey(), data[:]); err != nil {
		log.Crit("Failed to store main branch head data", "err", err)
	}
}

func ReadBloomSectionSavedFlag(db DatabaseReader, section uint64) bool {
	var result = false
	exist, _ := db.Has(bloomSectionSavedFlagKey(section))
	if exist {
		data, _ := db.Get(bloomSectionSavedFlagKey(section))
		if len(data) != 0 {
			err := rlp.DecodeBytes(data, &result)
			if err != nil {
				log.Error("ReadBloomSectionSavedFlag: DecodeBytes error: " + err.Error())
			}
		}
	}
	return result
}

func WriteBloomSectionSavedFlag(db DatabaseWriter, section uint64, flag bool) {
	log.Info("BlOOMDEBUG WriteBloomSectionSavedFlag", "section", section, "flag", flag)
	hashBytes, _ := rlp.EncodeToBytes(flag)
	if err := db.Put(bloomSectionSavedFlagKey(section), hashBytes); err != nil {
		log.Crit("Failed to store block state check flag", "err", err)
	}
}

func ReadBloomFastSyncReachHeight(db DatabaseReader) (uint64, error) {
	data, err := db.Get(bloomFastSyncReachHeightMapKey())
	if err != nil || len(data) != 8 {
		return 0, err
	}
	return binary.BigEndian.Uint64(data[:]), nil
}

func WriteBloomFastSyncReachHeight(db DatabaseWriter, height uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], height)
	if err := db.Put(bloomFastSyncReachHeightMapKey(), data[:]); err != nil {
		log.Crit("Failed to store bloom fast sync reach height", "err", err)
	}
}

func ReadPackerNonce(db DatabaseReader, coinbase common.Address) (uint64, error) {
	data, err := db.Get(txPackageNonceKey(coinbase))
	if err != nil || len(data) != 8 {
		return 0, err
	}
	return binary.BigEndian.Uint64(data[:]), nil
}

func WritePackerNonce(db DatabaseWriter, coinbase common.Address, nonce uint64) {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], nonce)
	if err := db.Put(txPackageNonceKey(coinbase), data[:]); err != nil {
		log.Crit("Failed to store packer nonce", "err", err)
	}
}

func ReadTxPkg(db DatabaseReader, hash common.Hash) (*types.TxPackage, error) {
	data, err := ReadTxPkgRLP(db, hash)
	if err != nil || len(data) == 0 {
		return nil, err
	}

	var pkg types.TxPackage
	err = rlp.DecodeBytes(data, &pkg)
	if err != nil {
		log.Crit("Failed to decode rlp bytes of txPackage.", "err", err)
		return nil, err
	}
	return &pkg, nil
}

func ReadTxPkgRLP(db DatabaseReader, hash common.Hash) (rlp.RawValue, error) {
	data, err := db.Get(txPackageHashKey(hash))
	return data, err
}

// HasTxPkg verifies the existence of a txpkg corresponding to the hash.
func HasTxPkg(db DatabaseReader, hash common.Hash) bool {
	if has, err := db.Has(txPackageHashKey(hash)); !has || err != nil {
		return false
	}
	return true
}

func WriteTxPkg(db DatabaseWriter, pkg *types.TxPackage) {
	// Write the encoded txPackage
	pkgBytes, err := rlp.EncodeToBytes(pkg)
	if err != nil {
		log.Crit("Failed to RLP encode TxPackage", "err", err)
	}
	dataKey := txPackageHashKey(pkg.Hash())
	if err := db.Put(dataKey, pkgBytes); err != nil {
		log.Crit("Failed to store TxPackage", "err", err)
	}
}
