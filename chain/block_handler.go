package chain

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/params"
	"math"
	"time"
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
	for _, confirmedBlock := range confirmedBlocks {
		state.ConfirmReward(stateDb, block, confirmedBlock)
	}
	state.MiningReward(stateDb, block)

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

	if bc.HasBlock(block.FullHash()) {
		bc.mu.Unlock()
		return
	}

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
	//dbaccessor.WriteBlockChilds(bc.db, block.FullHash(), []common.Hash{})

	// store block
	dbaccessor.WriteBlock(bc.db, block)

	// store parent-hash-childs
	childs := dbaccessor.ReadBlockChilds(bc.db, block.Header.ParentFullHash)
	childs = append(childs, block.FullHash())
	dbaccessor.WriteBlockChilds(bc.db, block.Header.ParentFullHash, childs)

	// unlock
	bc.mu.Unlock()

	//elapse := int64(block.ReceivedAt.UnixNano()/1e6) - int64(block.Header.MinedTime)
	//if elapse < 0 {
	//	elapse = 0
	//}
	bc.logger.Info("Insert block OK", "type", "console", "height", block.Header.Height, "round", block.Header.Round,
		"hash", block.FullHash(), "difficulty", block.Header.Difficulty,
		"txCount", len(block.Body.Transactions), "pkgCount", len(block.Body.TxPackageHashes))
	//bc.logger.Info("Block metric information", "hash", block.FullHash(),
	//	"duration", common.PrettyDuration(time.Since(block.ReceivedAt)), "elapse", elapse, "hop", block.Header.HopCount)
}

func (bc *BlockChain) checkCheckPoint(currentBlock *types.Block, block *types.Block) bool {
	if !bc.chainConfig.CheckPointEnable {
		return true
	}
	if block == nil {
		return false
	}
	// get checkPoint below currentBlock
	checkPoint := config.GetCheckPointBelowBlock(currentBlock, bc.checkPoints)
	bc.logger.Info("checkCheckPoint", "checkPoint", checkPoint, "block.height", block.Header.Height, "block.round", block.Header.Round, "block.hash", block.FullHash())
	if checkPoint.Height == block.Header.Height && checkPoint.Hash.String() == block.FullHash().String() {
		return true
	} else if block.Header.Height > checkPoint.Height {
		return true
	}
	return false
}

// insert block
func (bc *BlockChain) InsertBlock(block *types.Block) {
	bc.insertBlockIntoDB(block)
	bc.checkAndSetHead(block)

	//
	bc.chainUpdateFeed.Send(types.ChainUpdateEvent{Block: block})
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
	return nil
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

func (bc *BlockChain) findCommonPreFix(block1 *types.Block, block2 *types.Block) *types.Block {
	height1 := block1.Header.Height
	height2 := block2.Header.Height

	for height1 > height2 {
		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
	}
	for height2 > height1 {
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
	}

	for block1.FullHash() != block2.FullHash() {
		if height1 == 0 {
			return bc.genesisBlock
		}

		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
	}

	return block1
}
