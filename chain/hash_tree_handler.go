// Copyright 2019 The go-fractal Authors
// This file is part of the go-fractal library.

// hash_tree_handler.go is main entry for hash tree

package chain

import (
	"errors"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
	"github.com/golang-collections/collections/stack"
)

const (
	MarkUnCompleted = ^uint64(0) - 1
)

var (
	errBlockNotFound     = errors.New("block not found")
	errHeightDiffNotMeet = errors.New("block height diff not meet")
)

func (bc *BlockChain) checkHashTreeInput(belowBlockHash common.Hash, upBlockHash common.Hash) (*types.Block, *types.Block, error) {
	upBlock := bc.GetBlock(upBlockHash)
	if upBlock == nil {
		return nil, nil, errBlockNotFound
	}
	belowBlock := bc.GetBlock(belowBlockHash)
	if belowBlock == nil {
		return nil, nil, errBlockNotFound
	}

	if upBlock.Header.Height < belowBlock.Header.Height {
		return nil, nil, errHeightDiffNotMeet
	}

	if (upBlock.Header.Height - belowBlock.Header.Height) <= types.HashTreeMinLength {
		return nil, nil, errHeightDiffNotMeet
	}

	bc.logger.Info("hash tree", "fromHash", belowBlockHash, "fromHeight", belowBlock.Header.Height, "toHash", upBlockHash, "toHeight", upBlock.Header.Height)
	return upBlock, belowBlock, nil
}

func (bc *BlockChain) checkInTreePoint(blockHash common.Hash, belowHeight uint64) bool {
	block := bc.GetBlock(blockHash)

	if belowHeight == 0 {
		if block.Header.Height == 0 {
			return true
		} else {
			return false
		}
	} else {
		if block.Header.Height <= belowHeight+uint64(bc.chainConfig.Greedy) {
			return true
		} else {
			return false
		}
	}

}

func (bc *BlockChain) CreateHashTree(belowBlockHash common.Hash, upBlockHash common.Hash) (*types.HashTree, *types.TreePoint, error) {
	// check input
	upBlock, belowBlock, err := bc.checkHashTreeInput(belowBlockHash, upBlockHash)
	if err != nil {
		return nil, nil, err
	}

	markGenesis := false
	if belowBlockHash == bc.genesisBlock.FullHash() {
		markGenesis = true
	}

	belowHeight := belowBlock.Header.Height

	// hash tree's all elements
	var elems types.TreeElems

	// cache: full hash -> index
	hashIndexMap := make(map[common.Hash]uint64)

	// tree point
	preHeight := params.StakeRegisterHeightDistance
	if belowHeight < preHeight {
		preHeight = belowHeight
	}

	var treePointMainListLength uint64
	var treePointMainChainList common.Hashes
	if markGenesis {
		treePointMainChainList = append(treePointMainChainList, belowBlockHash)
	} else {
		treePointMainListLength = preHeight + uint64(bc.chainConfig.Greedy) + 1
		treePointMainChainList, err = bc.getTreePointHashList(upBlock.FullHash(), upBlock.Header.Height-belowBlock.Header.Height+preHeight+1, treePointMainListLength)
		if err != nil {
			return nil, nil, err
		}
	}

	treePointSet := mapset.NewSet()
	treePoint := types.TreePoint{
		Height:            belowHeight,
		FullHash:          belowBlockHash,
		MainChainHashList: treePointMainChainList,
	}

	//
	rootIndex := uint64(0)
	stack := stack.New()
	stack.Push(upBlockHash)
	for stack.Len() > 0 {
		elemHash := stack.Peek().(common.Hash)

		//already completed: in hashIndexMap and treeBody and parent is marked
		index, ok := hashIndexMap[elemHash]
		if ok && elems[index].ParentFullHash != MarkUnCompleted {
			stack.Pop()
			continue
		}

		block := bc.GetBlock(elemHash)
		if block == nil {
			return nil, nil, errBlockNotFound
		}

		var blockTreeElem *types.TreeElem
		if ok {
			blockTreeElem = elems[index]
		} else {
			blockTreeElem = &types.TreeElem{FullHash: block.FullHash(), ParentFullHash: MarkUnCompleted}
		}

		//in tree body
		//parent and confirms

		parentHash, confirms, err := bc.getParentAndConfirmsFromBlockChain(elemHash)
		if err != nil {
			return nil, nil, err
		}

		var unCompletedHashes []common.Hash
		for _, confirmHash := range confirms {
			if index, ok := hashIndexMap[confirmHash]; !ok || (ok && elems[index].ParentFullHash == MarkUnCompleted) {
				unCompletedHashes = append(unCompletedHashes, confirmHash)
			}
		}
		if index, ok := hashIndexMap[parentHash]; !ok || (ok && elems[index].ParentFullHash == MarkUnCompleted) {
			unCompletedHashes = append(unCompletedHashes, parentHash)
		}

		if len(unCompletedHashes) == 0 {
			blockTreeElem.ParentFullHash = hashIndexMap[parentHash]
			for _, confirmHash := range confirms {
				blockTreeElem.Confirms = append(blockTreeElem.Confirms, hashIndexMap[confirmHash])
			}

			//bc.logger.Info("create hash tree: parent and confirm ready", "treeElem", blockTreeElem, "len(elems)", len(elems), "elems", elems)

			if !ok {
				elems = append(elems, blockTreeElem)
				hashIndexMap[blockTreeElem.FullHash] = uint64(len(elems) - 1)
			}

			stack.Pop()
		} else {
			//append uncompleted hashes
			for _, unverifiedHash := range unCompletedHashes {
				var unverifiedTreeElem *types.TreeElem
				_, unverifiedOk := hashIndexMap[unverifiedHash]

				if bc.checkInTreePoint(unverifiedHash, belowHeight) {
					if !unverifiedOk {
						unverifiedTreeElem = &types.TreeElem{FullHash: unverifiedHash, ParentFullHash: types.MarkTreePoint}
					}
					// add parent until belowBlock
					unverifiedBlock := bc.GetBlock(unverifiedHash)
					bc.addParentListToTreePoint(&treePoint, treePointSet, unverifiedBlock, belowHeight)
				} else {

					if !unverifiedOk {
						unverifiedTreeElem = &types.TreeElem{FullHash: unverifiedHash, ParentFullHash: MarkUnCompleted}
						stack.Push(unverifiedHash)
					}
				}

				if !unverifiedOk {
					elems = append(elems, unverifiedTreeElem)
					hashIndexMap[unverifiedHash] = uint64(len(elems) - 1)
				}

				//bc.logger.Info("create hash tree: parent and confirm not ready", "unverifiedTreeElem", unverifiedTreeElem, "len(elems)", len(elems), "elems", elems)

			}

			blockTreeElem.ParentFullHash = hashIndexMap[parentHash]
			for _, confirmHash := range confirms {
				blockTreeElem.Confirms = append(blockTreeElem.Confirms, hashIndexMap[confirmHash])
			}

			if !ok {
				elems = append(elems, blockTreeElem)
				hashIndexMap[blockTreeElem.FullHash] = uint64(len(elems) - 1)
			}

			if blockTreeElem.FullHash == upBlockHash {
				rootIndex = uint64(len(elems)) - 1
			}
			//bc.logger.Info("create hash tree: after parent and confirm ready", "treeElem", blockTreeElem, "len(elems)", len(elems), "elems", elems)

		}
	}
	bc.logger.Info("create hash tree succeed", "rootIndex", rootIndex, "len(hashTree)",
		len(elems), "hashTree", elems, "belowHeight",
		treePoint.Height, "belowHash", treePoint.FullHash, "len(treePointMainChainList)", len(treePoint.MainChainHashList),
		"len(treePointHashPairs)", len(treePoint.HashPairs))
	return &types.HashTree{RootIndex: rootIndex, Elems: elems}, &treePoint, nil
}

func (bc *BlockChain) getParentAndConfirmsFromBlockChain(hash common.Hash) (common.Hash, common.Hashes, error) {
	block := bc.GetBlock(hash)
	if block == nil {
		return common.Hash{}, nil, errBlockNotFound
	}

	parentBlock := bc.GetBlock(block.Header.ParentFullHash)
	if parentBlock == nil {
		return common.Hash{}, nil, errBlockNotFound
	}

	var confirms common.Hashes
	for _, confirmHash := range block.Header.Confirms {
		confirmBlock := bc.GetBlock(confirmHash)
		if confirmBlock == nil {
			return common.Hash{}, nil, errBlockNotFound
		}
		confirms = append(confirms, confirmHash)
	}
	//bc.logger.Info("get parent and confirms", "hash", hash, "parent", parentBlock.FullHash(), "confirms", confirms)
	return parentBlock.FullHash(), confirms, nil
}

func (bc *BlockChain) getTreePointHashList(upHash common.Hash, length uint64, treePointLength uint64) ([]common.Hash, error) {
	block := bc.GetBlock(upHash)
	if block == nil {
		bc.logger.Error("block is nil", "hash", upHash)
		return nil, errors.New("block is nil")
	}

	var res []common.Hash
	res = append([]common.Hash{block.FullHash()}, res...)
	for i := uint64(0); i < length-1; i++ {
		hash := block.Header.ParentFullHash
		block = bc.GetBlock(hash)
		if block == nil {
			bc.logger.Error("block is nil", "hash", hash)
			return nil, errors.New("block is nil")
		}
		res = append([]common.Hash{block.FullHash()}, res...)
	}

	return res[0:treePointLength], nil
}

func (bc *BlockChain) addParentListToTreePoint(treePoint *types.TreePoint, treePointSet mapset.Set, block *types.Block, lowestHeight uint64) error {
	bc.logger.Info("add parent list to tree point", "blockHash", block.FullHash(), "blockHeight", block.Header.Height, "lowestHeight", lowestHeight)
	blockTemp := block

	for blockTemp.Header.Height >= lowestHeight && !treePointSet.Contains(blockTemp.FullHash()) {
		if _, ok := treePoint.RetrieveAccHashMap()[blockTemp.FullHash()]; ok {
			return nil
		}

		treePoint.HashPairs = append(treePoint.HashPairs, types.HashPairFullAcc{blockTemp.FullHash(), blockTemp.AccHash})
		treePointSet.Add(blockTemp.FullHash())

		parentBlock := bc.GetBlock(blockTemp.Header.ParentFullHash)
		if parentBlock == nil {
			return errBlockNotFound
		}
		blockTemp = parentBlock
	}
	return nil
}
