package chain

import (
	"fmt"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils"
)

// CheckGreedy checks greedy rules
func (bc *BlockChain) CheckGreedy(block *types.Block, mainBlock *types.Block, greedy uint64) (bool, error) {
	for mainBlock.CompareByRoundAndSimpleHash(block) > 0 {
		mainBlock = bc.GetBlock(mainBlock.Header.ParentFullHash)
		if mainBlock == nil {
			return false, fmt.Errorf("block not find when check greedy: %s", mainBlock.Header.ParentFullHash)
		}
	}

	// if height diff is larger then greedy, then return false directly
	heightDiff := utils.Abs(int64(block.Header.Height) - int64(mainBlock.Header.Height))
	if heightDiff > int64(greedy) {
		return false, nil
	}

	hopCount, err := bc.GetHopCount(block, mainBlock)
	if err != nil {
		return false, err
	}
	if hopCount > greedy+1 {
		return false, nil
	}
	return true, nil
}

func (bc *BlockChain) GetHopCount(block1 *types.Block, block2 *types.Block) (uint64, error) {
	height1 := block1.Header.Height
	height2 := block2.Header.Height

	var hopCount uint64

	for height1 > height2 {
		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
		hopCount++
	}

	for height2 > height1 {
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
		hopCount++
	}

	for block1.FullHash() != block2.FullHash() {
		if height1 == 0 {
			return 0, ErrMoreThanOneGenesis
		}

		block1 = bc.GetBlock(block1.Header.ParentFullHash)
		height1--
		block2 = bc.GetBlock(block2.Header.ParentFullHash)
		height2--
		hopCount = hopCount + 2
	}

	return hopCount, nil
}

// GetGreedyBlocks returns the block for mining with greedy-param.
func (bc *BlockChain) GetGreedyBlocks(greedy uint8) (blocks types.Blocks) {
	visitedBlockHashes := mapset.NewSet()

	var currentBlocks types.Blocks
	currentBlock := bc.CurrentBlock()
	// if current block is lower than the most top ,no need to mine block
	// TODO
	//if bc.chainConfig.CheckPointEnable {
	//	_, height := bc.GetLocalLatestCheckPoint()
	//	if currentBlock.Header.Height < height {
	//		bc.logger.Info("current block is lower than the most top checkpoints, no need to mine block")
	//		return currentBlocks
	//	}
	//}
	currentBlocks = append(currentBlocks, currentBlock)
	blocks = append(blocks, currentBlock)
	visitedBlockHashes.Add(currentBlock.FullHash())

	for i := uint8(0); i < greedy; i++ {
		var nextLoopBlocks types.Blocks
		for _, currentBlock := range currentBlocks {
			parentBlock := bc.GetBlock(currentBlock.Header.ParentFullHash)
			if parentBlock != nil {
				if !visitedBlockHashes.Contains(parentBlock.FullHash()) {
					visitedBlockHashes.Add(parentBlock.FullHash())
					nextLoopBlocks = append(nextLoopBlocks, parentBlock)
					blocks = append(blocks, parentBlock)
				}
			}

			childs := bc.GetBlockChilds(currentBlock.FullHash())
			for _, child := range childs {
				if !visitedBlockHashes.Contains(child) {
					visitedBlockHashes.Add(child)
					childBlock := bc.GetBlock(child)
					nextLoopBlocks = append(nextLoopBlocks, childBlock)
					blocks = append(blocks, childBlock)
				}
			}
		}

		currentBlocks = nextLoopBlocks
	}
	return
}
