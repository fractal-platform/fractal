package chain

import (
	"math"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/params"
)

func (bc *BlockChain) GetBlockBeforeCacheHeight(block *types.Block, cacheHeight uint8) (*types.Block, bool) {
	temp := block
	for i := uint8(0); i < cacheHeight; i++ {
		preBlock := bc.GetBlock(temp.Header.ParentFullHash)
		if preBlock == nil {
			bc.logger.Info("GetBlockBeforeCacheHeight miss block", "blockHash", temp.FullHash(), "height", temp.Header.Height, "ParentFullHash", temp.Header.ParentFullHash)
			return nil, false
		}
		temp = preBlock
		if temp.FullHash() == bc.genesisBlock.FullHash() {
			return temp, true
		}
	}

	return temp, true
}

func (bc *BlockChain) Filter(hashes common.Hashes) common.Hashes {
	var res common.Hashes
	for _, hash := range hashes {
		stateEnum := dbaccessor.ReadBlockStateCheck(bc.db, hash)
		if stateEnum == types.NoBlockState {
			res = append(res, hash)
		}
	}
	return res
}

func (bc *BlockChain) execBlock(block *types.Block, confirmedBlocks types.Blocks) (*state.StateDB, types.Receipts, []*types.TxWithIndex, *types.Bloom, error) {
	var (
		//prevStateDb *state.StateDB
		stateDb  *state.StateDB
		receipts types.Receipts
	)
	bc.logger.Info("Block TxExec Verify Start", "hash", block.FullHash(), "parent", block.Header.ParentFullHash, "duration", common.PrettyDuration(time.Since(block.ReceivedAt)), "pkgNum", len(block.Body.TxPackageHashes), "height", block.Header.Height)

	//
	parentBlock := bc.GetBlock(block.Header.ParentFullHash)
	stateDb, _ = bc.StateAt(parentBlock.Header.StateHash)

	var (
		executedTxs []*types.TxWithIndex
		allLogs     []*types.Log
		usedGas     = new(uint64)
		gasPool     = new(types.GasPool).AddGas(math.MaxUint64)
		txpkgs      = bc.GetTxPackageList(block.Body.TxPackageHashes)
	)

	prevStateDb, _, _ := bc.GetStateBeforeCacheHeight(parentBlock, uint8(params.ConfirmHeightDistance-1))
	callbackParamKey := wasm.GetGlobalRegisterParam().RegisterParam(stateDb, block)
	executedTxs, allLogs, receipts, _ = bc.txExecutor.ExecuteTxPackages(txpkgs, prevStateDb, stateDb, receipts, block, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)
	executedTxs, _, receipts, _ = bc.txExecutor.ExecuteTransactions(block.Body.Transactions, prevStateDb, stateDb, receipts, block, types.NotInPackage, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)
	wasm.GetGlobalRegisterParam().UnRegisterParam(callbackParamKey)
	bc.logger.Info("finish executing packages and transactions", "hash", block.FullHash(), "executedTxs", len(executedTxs), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	if *usedGas != block.Header.GasUsed {
		return nil, nil, nil, nil, ErrInvalidGasUsed
	}

	// set reward
	state.AddBlockReward(stateDb, block, confirmedBlocks)

	stateDb.Finalise(true)
	bc.logger.Info("finish finalising statedb", "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	var receiptHash common.Hash
	var bloom *types.Bloom
	if len(receipts) == 0 {
		receiptHash = types.DeriveSha(types.Receipts{})
		bloom = &types.Bloom{}
	} else {
		receiptHash = types.DeriveSha(types.Receipts(receipts))
		bloom = types.CreateBloom(receipts)
	}
	block.CacheBloom(bloom)

	if receiptHash != block.Header.ReceiptHash {
		return nil, nil, nil, nil, ErrBlockReceiptError
	}

	newStateHash := stateDb.IntermediateRoot(true)
	if newStateHash != block.Header.StateHash {
		// dump it to db, for debug use
		root, err := stateDb.Commit(true)
		if err != nil {
			bc.logger.Error("Commit bad state failed(state commit error)", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash(), "err", err)
		}
		if err = bc.stateCache.TrieDB().Commit(root, false); err != nil {
			bc.logger.Error("Commit bad state failed(trieDB commit error)", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash(), "err", err)
		}
		bc.logger.Error("state check failed", "newStateHash", newStateHash, "blockStateHash", block.Header.StateHash)

		return nil, nil, nil, nil, ErrBlockStateError
	}

	bc.logger.Info("Block TxExec Verify OK", "hash", block.FullHash(), "state", block.Header.StateHash, "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
	return stateDb, receipts, executedTxs, bloom, nil
}

// insert block
func (bc *BlockChain) insertBlockIntoDB(block *types.Block) {
	// lock
	bc.mu.Lock()

	// store round-hash-list
	hashList := dbaccessor.ReadHashListByRound(bc.db, block.Header.Round)
	hashList = append(hashList, &types.BlockRoundHash{
		Round:      block.Header.Round,
		SimpleHash: block.SimpleHash(),
		FullHash:   block.FullHash(),
	})
	hashList.SortByRoundHash()
	dbaccessor.WriteHashList(bc.db, block.Header.Round, hashList)

	// store hash-childs
	dbaccessor.WriteBlockChilds(bc.db, block.FullHash(), []common.Hash{})

	// set accHash
	bc.CalcuAccHash(block)
	// store block
	dbaccessor.WriteBlock(bc.db, block)

	// store parent-hash-childs
	childs := dbaccessor.ReadBlockChilds(bc.db, block.Header.ParentFullHash)
	childs = append(childs, block.FullHash())
	dbaccessor.WriteBlockChilds(bc.db, block.Header.ParentFullHash, childs)

	// unlock
	bc.mu.Unlock()

	elapse := int64(block.ReceivedAt.UnixNano()/1e6) - int64(block.Header.MinedTime)
	if elapse < 0 {
		elapse = 0
	}
	bc.logger.Info("Insert block OK", "type", "console", "height", block.Header.Height, "round", block.Header.Round,
		"hash", block.FullHash(), "difficulty", block.Header.Difficulty,
		"txCount", len(block.Body.Transactions), "pkgCount", len(block.Body.TxPackageHashes), "parentHash", block.Header.ParentFullHash)
	bc.logger.Info("Block metric information", "hash", block.FullHash(),
		"duration", common.PrettyDuration(time.Since(block.ReceivedAt)), "elapse", elapse, "hop", block.Header.HopCount)
}

func (bc *BlockChain) InsertBlock(block *types.Block) {
	// insert
	bc.insertBlockIntoDB(block)
	bc.checkAndSetHead(block)

	//
	bc.chainUpdateFeed.Send(types.ChainUpdateEvent{Block: block})

	// process future Blocks
	for _, futureBlock := range bc.getFutureBlocks(block.FullHash()) {
		bc.logger.Info("Process future block", "hash", futureBlock.FullHash(), "height", futureBlock.Header.Height, "depend", block.FullHash())
		bc.futureBlockFeed.Send(types.FutureBlockEvent{Block: futureBlock})
	}
	bc.removeFutureBlocks(block.FullHash())

	// process future tx packages
	for _, futureTxPackage := range bc.getFutureBlockTxPackages(block.FullHash()) {
		bc.logger.Info("Process future txpkg", "pkgHash", futureTxPackage.Hash(), "blockHash", block.FullHash())
		bc.removeFutureBlockTxPackage(futureTxPackage.Hash())
		bc.futureTxPackageFeed.Send(types.FutureTxPackageEvent{Pkg: futureTxPackage})
	}
}

// insert block before current block
func (bc *BlockChain) InsertPastBlock(block *types.Block) error {
	// TODO: checkpoint also return nil
	if block.FullHash() == bc.genesisBlock.FullHash() {
		return nil
	}

	bc.insertBlockIntoDB(block)
	var confirmBlocks types.Blocks
	for _, fullHash := range block.Header.Confirms {
		var confirmBlock = bc.GetBlock(fullHash)
		confirmBlocks = append(confirmBlocks, confirmBlock)
	}
	state, receipts, executedTxs, bloom, err := bc.execBlock(block, confirmBlocks)
	if err != nil {
		return err
	} else {
		bc.insertBlockState(block, state, receipts, executedTxs, bloom)
	}
	dbaccessor.WriteHeadBlockHash(bc.db, block.FullHash())
	return nil
	//
	//bc.chainUpdateFeed.Send(types.ChainUpdateEvent{block})
}

// insert block
func (bc *BlockChain) InsertBlockNoCheck(block *types.Block) {
	bc.insertBlockIntoDB(block)
}

//
func (bc *BlockChain) InsertBlockWithState(block *types.Block, state *state.StateDB, receipts types.Receipts, executedTxs []*types.TxWithIndex, bloom *types.Bloom) {
	bc.insertBlockIntoDB(block)
	bc.insertBlockState(block, state, receipts, executedTxs, bloom)
	bc.checkAndSetHead(block)

	//
	bc.chainUpdateFeed.Send(types.ChainUpdateEvent{Block: block})
}

// the common prefix block height must >= height limit
func (bc *BlockChain) findCommonPrefixWithHeightLimit(block1 *types.Block, block2 *types.Block, limit uint64) (*types.Block, bool) {
	height1 := block1.Header.Height
	height2 := block2.Header.Height

	if limit > height1 || limit > height2 {
		return nil, false
	}

	for height1 > height2 {
		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
	}
	for height2 > height1 {
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
	}

	for block1.FullHash() != block2.FullHash() {
		if height1 == limit {
			return nil, false
		}

		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
	}

	return block1, true
}
