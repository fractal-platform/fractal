// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// blockchain check contains the implementation of fractal chain check.

package chain

import (
	"errors"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
)

func (bc *BlockChain) GetBreakPoint(checkpoint *types.Block, headBlock *types.Block) (*types.Block, *types.Block, error) {
	var blockFrom *types.Block = nil
	var blockTo *types.Block = nil
	//check args
	if checkpoint == nil {
		//block = bc.GetBlock(config.GetLatestCheckPoint(bc.checkPoints).Hash)
		blockFrom = bc.genesisBlock
	} else {
		blockFrom = bc.GetBlock(checkpoint.FullHash())
	}
	if headBlock == nil {
		blockTo = bc.currentBlock.Load().(*types.Block)
	} else {
		blockTo = bc.GetBlock(headBlock.FullHash())
	}
	if blockFrom == nil || blockTo == nil || blockFrom.Header.Height >= blockTo.Header.Height {
		return nil, nil, errors.New("args error")
	}
	_, err := bc.StateAt(blockFrom.Header.StateHash)
	if err != nil {
		return nil, nil, errors.New("below block state not exist")
	}
	_, err = bc.StateAt(blockTo.Header.StateHash)
	if err != nil {
		return nil, nil, errors.New("above block state not exist")
	}

	//get below breakpoint
	currentBlocks := []common.Hash{blockFrom.FullHash()}
	parentBlock := blockFrom

	for {
		if len(currentBlocks) <= 0 {
			break
		}
		var nextBlocks []common.Hash
		var currBlock *types.Block
		existAndRight := 0
		for _, curr := range currentBlocks {
			currBlock = bc.GetBlock(curr)
			if !bc.IsInMainBranch(currBlock) {
				continue
			}

			_, err = bc.StateAt(blockTo.Header.StateHash)
			if err != nil {
				break
			}
			_, err = dbaccessor.ReadBloom(bc.db, blockTo.FullHash())
			if err != nil {
				break
			}
			//
			if currBlock.Header.Height >= blockTo.Header.Height {
				return nil, nil, nil
			}
			childs := dbaccessor.ReadBlockChilds(bc.db, curr)
			nextBlocks = append(nextBlocks, childs...)
			existAndRight = 1
			break
		}
		if existAndRight == 0 {
			break
		}
		currentBlocks = nextBlocks
		parentBlock = currBlock
	}

	//get above breakpoint
	childBlock := blockTo
	for childBlock != nil {
		block := bc.GetBlock(childBlock.Header.ParentFullHash)
		if block == nil {
			break
		}

		_, err = bc.StateAt(block.Header.StateHash)
		if err != nil {
			break
		}
		_, err = dbaccessor.ReadBloom(bc.db, block.FullHash())
		if err != nil {
			break
		}

		if block.Header.Height <= blockFrom.Header.Height {
			return nil, nil, nil
		}

		childBlock = block
	}

	return parentBlock, childBlock, nil
}

func (bc *BlockChain) CheckBlocksReverse() ([]common.Hash, map[common.Hash]common.Hash, bool) {
	emptyHash := common.Hash{}
	missParentBlocks := make([]common.Hash, 0)
	missPkgBlocks := make(map[common.Hash]common.Hash)
	check := true

	block := bc.CurrentBlock()
	if block == nil {
		return missParentBlocks, missPkgBlocks, false
	}
	var currentBlocks = []common.Hash{block.FullHash()}
	genesisHash := bc.genesisBlock.FullHash()
	verified := make(map[common.Hash]struct{})

	for {
		if len(currentBlocks) <= 0 {
			break
		}

		nextBlocks := make([]common.Hash, 0)
		for _, current := range currentBlocks {
			if current == genesisHash {
				break
			}

			currBlock := bc.GetBlock(current)
			confirmBlocks, dependParentHash, dependPkgHash, err := bc.VerifyBlock(currBlock, true)
			if err != nil {
				check = false
			}
			if dependParentHash != emptyHash {
				missParentBlocks = append(missParentBlocks, current)
			} else {
				if _, ok := verified[currBlock.Header.ParentFullHash]; !ok {
					nextBlocks = append(nextBlocks, currBlock.Header.ParentFullHash)
				}
			}
			if dependPkgHash != emptyHash {
				missPkgBlocks[current] = dependPkgHash
			}

			verified[current] = struct{}{}
			for _, confirmBlock := range confirmBlocks {
				if _, ok := verified[confirmBlock.FullHash()]; !ok {
					nextBlocks = append(nextBlocks, confirmBlock.FullHash())
				}
			}
		}
		currentBlocks = nextBlocks
	}
	return missParentBlocks, missPkgBlocks, check
}

func (bc *BlockChain) CheckStateTrieFrom(block *types.Block) common.Hash {
	var currentBlockHashList = []common.Hash{block.FullHash()}
	for {
		if len(currentBlockHashList) <= 0 {
			break
		}

		var nextBlockHashList []common.Hash
		for _, currentBlockHash := range currentBlockHashList {
			currBlock := bc.GetBlock(currentBlockHash)
			err := bc.checkStateTrie(currBlock)
			if err != nil {
				// TODO
				return currentBlockHash
			}
			childs := dbaccessor.ReadBlockChilds(bc.db, currentBlockHash)
			nextBlockHashList = append(nextBlockHashList, childs...)
		}
		currentBlockHashList = nextBlockHashList
	}
	return common.Hash{}
}

func (bc *BlockChain) checkStateTrie(block *types.Block) error {
	stateDB, err := bc.StateAt(block.Header.StateHash)
	if err != nil {
		return err
	}
	root := stateDB.GetRoot()
	tr, err := bc.stateCache.OpenTrie(root)
	if err != nil {
		return err
	}
	iter := tr.NodeIterator(nil)
	for iter.Next(true) {
		//if iter.Error() != nil {
		//	log.Error("CheckStateTrie", "blockhash", block.FullHash(), "err", err)
		//	return err
		//}
		//// TODO
	}
	if iter.Error().Error() != "end of iteration" {
		return iter.Error()
	}
	return nil
}

func (bc *BlockChain) CheckStateFrom(block *types.Block) (common.Hash, error) {
	var currentBlockHashList = []common.Hash{block.FullHash()}
	for {
		if len(currentBlockHashList) <= 0 {
			break
		}

		var nextBlockHashList []common.Hash
		for _, currentBlockHash := range currentBlockHashList {
			currBlock := bc.GetBlock(currentBlockHash)
			var confirmBlocks types.Blocks
			for _, fullHash := range currBlock.Header.Confirms {
				var confirmBlock = bc.GetBlock(fullHash)
				confirmBlocks = append(confirmBlocks, confirmBlock)
			}
			_, _, _, _, err := bc.execBlock(currBlock, confirmBlocks)
			if err != nil {
				return currentBlockHash, err
			}
			childs := dbaccessor.ReadBlockChilds(bc.db, currentBlockHash)
			nextBlockHashList = append(nextBlockHashList, childs...)
		}
		currentBlockHashList = nextBlockHashList
	}
	return common.Hash{}, nil
}
