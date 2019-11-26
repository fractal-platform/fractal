package chain

import (
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils"
	"github.com/rcrowley/go-metrics"
)

// StateAt returns a new mutable state based on a particular point in time.
func (bc *BlockChain) StateAt(stateHash common.Hash) (*state.StateDB, error) {
	return state.New(stateHash, bc.stateCache)
}

func (bc *BlockChain) GetStateBeforeCacheHeight(block *types.Block, cacheHeight uint8) (*state.StateDB, *types.Block, bool) {
	if uint64(cacheHeight) > block.Header.Height {
		cacheHeight = uint8(block.Header.Height)
	}

	block, ok := bc.GetBlockBeforeCacheHeight(block, cacheHeight)
	if !ok {
		return nil, nil, false
	}
	stateDb, err := bc.StateAt(block.Header.StateHash)
	if err != nil {
		bc.logger.Error("Get state before cache height failed", "blockhash", block.FullHash(), "err", err)
		return nil, nil, false
	}

	return stateDb, block, true
}

func (bc *BlockChain) GetPreBalanceAndPubkey(block *types.Block, address common.Address) (uint64, []byte, error) {
	stateDb, _, _ := bc.GetStateBeforeCacheHeight(block, uint8(params.StakeRegisterHeightDistance))
	if stateDb == nil {
		return 0, []byte{}, ErrBlockStateNotFound
	}

	balance := stateDb.GetBalance(address)

	var pubkey []byte
	table, _ := utils.String2Uint64(params.MinerKeyContractTable)
	storageKey := state.GetStorageKey(table, address[:])
	storageBytes := stateDb.GetState(common.HexToAddress(params.MinerKeyContractAddr), storageKey)
	if storageBytes != nil {
		pubkey = storageBytes[22:]
	}

	return balance.Uint64(), pubkey, nil
}

func (bc *BlockChain) getPackerConfirmState(headBlockWhenPacking *types.Block) (*state.StateDB, *types.Block, error) {
	distance := uint64(bc.GetGreedy()) + params.PackerKeyConfirmDistance
	if headBlockWhenPacking == nil {
		return nil, nil, ErrBlockNotFound
	}
	if distance > headBlockWhenPacking.Header.Height {
		distance = headBlockWhenPacking.Header.Height
	}
	stateDb, block, _ := bc.GetStateBeforeCacheHeight(headBlockWhenPacking, uint8(distance))
	if stateDb == nil {
		return nil, nil, ErrBlockStateNotFound
	}

	return stateDb, block, nil
}

func (bc *BlockChain) GetPrePackerInfoByIndex(headBlockWhenPacking *types.Block, index uint32) (*types.PackerInfo, *types.Block, error) {
	stateDb, block, err := bc.getPackerConfirmState(headBlockWhenPacking)
	if err != nil {
		return nil, nil, err
	}

	packerInfo := stateDb.GetPackerInfo(index)
	if packerInfo == nil {
		bc.logger.Info("packer info not found", "block", block.FullHash())
		return nil, nil, ErrPackerInfoNotFound
	}
	return packerInfo, block, nil
}

func (bc *BlockChain) GetPackerInfoByPubKey(blockWhenPacking *types.Block, pubKey types.PackerECPubKey) (uint32, uint32, *types.PackerInfo, error) {
	// Read cache first
	if packerInfoMap, err := bc.packerInfoMapCache.Get(blockWhenPacking.FullHash()); err == nil {
		bc.logger.Debug("GetPrePackerInfoByPubKey: read map", "hash", blockWhenPacking.FullHash(), "height", blockWhenPacking.Header.Height)
		// in cache
		index, exist := packerInfoMap.PubKeyIndexMap[pubKey]
		if !exist {
			bc.logger.Error("GetPrePackerInfoByPubKey: cannot find index in packerInfoMap")
			return 0, 0, nil, ErrPackerInfoNotFound
		}
		packerInfo, exist := packerInfoMap.IndexPackerMap[index]
		if !exist {
			bc.logger.Error("GetPrePackerInfoByPubKey: cannot find packerInfo in packerInfoMap")
			return 0, 0, nil, ErrPackerInfoNotFound
		}
		packerNumber := uint32(len(packerInfoMap.PubKeyIndexMap))
		return index, packerNumber, packerInfo, nil
	}

	// Read storage and then save to cache
	stateDb, err := bc.StateAt(blockWhenPacking.Header.StateHash)
	if err != nil {
		bc.logger.Error("GetPrePackerInfoByPubKey: trie node error", "err", err)
		return 0, 0, nil, err
	}
	packerNumber := stateDb.GetPackerNumber()
	if packerNumber == 0 {
		return 0, 0, nil, ErrPackerNumberIsZero
	}
	var result *types.PackerInfo
	var index uint32
	var packerInfoMap = types.NewPackerInfoMap()
	for index = 0; index < packerNumber; index++ {
		packerInfo := stateDb.GetPackerInfo(index)
		if packerInfo == nil {
			bc.logger.Error("GetPrePackerInfoByPubKey: packInfo in storage is nil", "index", index)
			continue
		}
		if packerInfo.PackerPubKey == pubKey {
			result = packerInfo
		}
		packerInfoMap.IndexPackerMap[index] = packerInfo
		packerInfoMap.PubKeyIndexMap[packerInfo.PackerPubKey] = index
	}

	// cache
	bc.packerInfoMapCache.Put(blockWhenPacking.FullHash(), packerInfoMap)

	if result == nil {
		bc.logger.Error("GetPrePackerInfoByPubKey: cannot find packerInfo in storage")
		return 0, 0, nil, ErrPackerInfoNotFound
	}

	return index, packerNumber, result, nil
}

func (bc *BlockChain) GetPrePackerNumber(headBlockWhenPacking *types.Block) (uint32, error) {
	stateDb, _, err := bc.getPackerConfirmState(headBlockWhenPacking)
	if err != nil {
		return 0, err
	}

	packerNumber := stateDb.GetPackerNumber()
	if packerNumber == 0 {
		return 0, ErrPackerNumberIsZero
	}

	return packerNumber, nil
}

//
func (bc *BlockChain) insertBlockState(block *types.Block, state *state.StateDB, receipts types.Receipts, executedTxs []*types.TxWithIndex, bloom *types.Bloom) error {
	// store executedTxs
	//start := time.Now()
	//dbaccessor.WriteTxLookupEntries(bc.db, block.FullHash(), executedTxs)
	//log.Info("InsertBlock:WriteTxLookupEntries duration", "executed tx num", len(executedTxs), "duration", time.Since(start))
	bc.txInChainProcessor.AddBlock(&blockWithExecutedTx{
		block:       block,
		executedTxs: executedTxs,
	})

	// store stateDB
	root, err := state.Commit(true)
	if err != nil {
		bc.logger.Error("Insert block failed(state commit error)", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash(), "err", err)
		return err
	}
	if err = bc.stateCache.TrieDB().Commit(root, false); err != nil {
		bc.logger.Error("Insert block failed(trieDB commit error)", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash(), "err", err)
		return err
	}

	// store bloom
	dbaccessor.WriteBloom(bc.db, block.FullHash(), bloom)

	// store receipts
	dbaccessor.WriteReceipts(bc.db, block.FullHash(), receipts)

	// remove from future
	bc.removeFutureBlock(block.FullHash())
	bc.removeFutureTxPackageBlock(block.FullHash())

	// set block state check flag
	dbaccessor.WriteBlockStateCheck(bc.db, block.FullHash(), types.BlockStateChecked)
	block.StateChecked = types.BlockStateChecked

	elapse := int64(block.ReceivedAt.UnixNano()/1e6) - int64(block.Header.MinedTime)
	if elapse < 0 {
		elapse = 0
	}
	bc.logger.Info("Insert block state OK", "height", block.Header.Height, "round", block.Header.Round, "state", block.Header.StateHash,
		"hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)), "elapse", elapse, "hop", block.Header.HopCount,
		"txCount", len(block.Body.Transactions), "pkgCount", len(block.Body.TxPackageHashes), "blockMinedTime", block.Header.MinedTime)

	bc.blockExecutedFeed.Send(types.BlockExecutedEvent{Block: block})

	if !metrics.UseNilMetrics {
		blockInsertTimer.UpdateSince(block.ReceivedAt)
		transactionInsertMeter.Mark(int64(len(block.Body.Transactions)))
		blockNetWorkDelayHistogram.Update(int64(elapse))
		blockInsertRevDelayHistogram.Update(int64(time.Since(block.ReceivedAt) / 1e6))
		blockInsertHopCountHistogram.Update(int64(block.Header.HopCount))

		for _, txPackageHash := range block.Body.TxPackageHashes {
			bc.logger.Debug("pkg in block", "hash", txPackageHash)
			txPackage := bc.GetTxPackage(txPackageHash)
			if txPackage != nil {
				transactionInsertMeter.Mark(int64(len(txPackage.Transactions())))
			} else {
				bc.logger.Error("pkg not in store?????", "hash", txPackageHash)
			}
		}
	}
	return nil
}
