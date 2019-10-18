// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package chain contains implementations for basic chain operations.
package chain

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

/////////////////// future block -> dependHash ///////////////////////////////////////////////////
// add block to future block, since [block] depends on [dependHash]
func (bc *BlockChain) addFutureBlock(dependHash common.Hash, block *types.Block) {
	bc.futureBlocksMutex.Lock()
	_, ok := bc.futureBlocks[dependHash]
	if !ok {
		blocks := make(types.Blocks, 0)
		bc.futureBlocks[dependHash] = &blocks
	}
	if !bc.futureBlocks[dependHash].Has(block.FullHash()) {
		*bc.futureBlocks[dependHash] = append(*bc.futureBlocks[dependHash], block)
	}
	bc.futureBlocksMutex.Unlock()

	log.Info("add future block -> block", "size", bc.futureBlocksSize(), "dependHash", dependHash, "blockHash", block.FullHash())
}

// remove the block from map item value(type.Blocks), when the block is handled already
func (bc *BlockChain) removeFutureBlock(hash common.Hash) {
	bc.futureBlocksMutex.Lock()
	for _, futureBlocks := range bc.futureBlocks {
		futureBlocks.Remove(hash)
	}
	bc.futureBlocksMutex.Unlock()
}

func (bc *BlockChain) futureBlocksSize() int {
	bc.futureBlocksMutex.RLock()
	defer bc.futureBlocksMutex.RUnlock()

	var count = 0
	for _, v := range bc.futureBlocks {
		count += len(*v)
	}
	return count
}

// get future blocks for [relatedHash]
func (bc *BlockChain) FutureBlocks(relatedHash common.Hash) types.Blocks {
	bc.futureBlocksMutex.RLock()
	defer bc.futureBlocksMutex.RUnlock()
	if blocks, ok := bc.futureBlocks[relatedHash]; ok {
		return blocks.Copy()
	}
	return types.Blocks{}
}

// remove future blocks for [relatedHash]
func (bc *BlockChain) RemoveFutureBlocks(relatedHash common.Hash) {
	bc.futureBlocksMutex.Lock()
	defer bc.futureBlocksMutex.Unlock()
	delete(bc.futureBlocks, relatedHash)
}

/////////////////// future block -> pkgHash ///////////////////////////////////////////////////
// add txpkg to future block, since [block] depends on [pkgHash]
func (bc *BlockChain) addFutureTxPackageBlock(pkgHash common.Hash, block *types.Block) {
	bc.futureTxPackageBlocksMutex.Lock()
	_, ok := bc.futureTxPackageBlocks[pkgHash]
	if !ok {
		blocks := make(types.Blocks, 0)
		bc.futureTxPackageBlocks[pkgHash] = &blocks
	}
	if !bc.futureTxPackageBlocks[pkgHash].Has(block.FullHash()) {
		*bc.futureTxPackageBlocks[pkgHash] = append(*bc.futureTxPackageBlocks[pkgHash], block)
	}
	bc.futureTxPackageBlocksMutex.Unlock()

	log.Info("add future tx package -> block", "size", bc.futureTxPackageBlocksSize(), "pkgHash", pkgHash, "blockHash", block.FullHash())
}

// remove the block from map item value(type.Blocks), when the block is handled already
func (bc *BlockChain) removeFutureTxPackageBlock(blockHash common.Hash) {
	bc.futureTxPackageBlocksMutex.Lock()
	for _, futureTxPackageBlocks := range bc.futureTxPackageBlocks {
		futureTxPackageBlocks.Remove(blockHash)
	}
	bc.futureTxPackageBlocksMutex.Unlock()
}

func (bc *BlockChain) futureTxPackageBlocksSize() int {
	bc.futureTxPackageBlocksMutex.RLock()
	defer bc.futureTxPackageBlocksMutex.RUnlock()

	var count = 0
	for _, v := range bc.futureTxPackageBlocks {
		count += len(*v)
	}
	return count
}

// get future txpkg blocks for [relatedTxPackageHash]
func (bc *BlockChain) FutureTxPackageBlocks(relatedTxPackageHash common.Hash) types.Blocks {
	bc.futureTxPackageBlocksMutex.RLock()
	defer bc.futureTxPackageBlocksMutex.RUnlock()
	if blocks, ok := bc.futureTxPackageBlocks[relatedTxPackageHash]; ok {
		return blocks.Copy()
	}
	return types.Blocks{}
}

// remove future txpkg blocks for [relatedTxPackageHash]
func (bc *BlockChain) RemoveFutureTxPackageBlocks(relatedTxPackageHash common.Hash) {
	bc.futureTxPackageBlocksMutex.Lock()
	defer bc.futureTxPackageBlocksMutex.Unlock()
	delete(bc.futureTxPackageBlocks, relatedTxPackageHash)
}

/////////////////// future txpkg -> blockHash ///////////////////////////////////////////////////
// add block to future txpkg, since [txpkg] depends on [blockHash]
func (bc *BlockChain) addFutureBlockTxPackage(blockHash common.Hash, pkg *types.TxPackage) {
	bc.futureBlockTxPackagesMutex.Lock()
	_, ok := bc.futureBlockTxPackages[blockHash]
	if !ok {
		pkgs := make(types.TxPackages, 0)
		bc.futureBlockTxPackages[blockHash] = &pkgs
	}
	if !bc.futureBlockTxPackages[blockHash].Has(pkg.Hash()) {
		*bc.futureBlockTxPackages[blockHash] = append(*bc.futureBlockTxPackages[blockHash], pkg)
	}
	bc.futureBlockTxPackagesMutex.Unlock()

	log.Info("add future block -> tx package", "size", bc.futureBlockTxPackagesSize(), "blockHash", pkg.BlockFullHash(), "pkgHash", pkg.Hash())
}

// remove the txpkg from map item value(type.Blocks), when the txpkg is handled already
func (bc *BlockChain) RemoveFutureBlockTxPackage(pkgHash common.Hash) {
	bc.futureBlockTxPackagesMutex.Lock()
	for _, futureBlockTxPackages := range bc.futureBlockTxPackages {
		futureBlockTxPackages.Remove(pkgHash)
	}
	bc.futureBlockTxPackagesMutex.Unlock()
}

func (bc *BlockChain) futureBlockTxPackagesSize() int {
	bc.futureBlockTxPackagesMutex.RLock()
	defer bc.futureBlockTxPackagesMutex.RUnlock()

	var count = 0
	for _, v := range bc.futureBlockTxPackages {
		count += len(*v)
	}
	return count
}

func (bc *BlockChain) FutureBlockTxPackages(blockHash common.Hash) types.TxPackages {
	bc.futureBlockTxPackagesMutex.RLock()
	defer bc.futureBlockTxPackagesMutex.RUnlock()
	if txpkgs, ok := bc.futureBlockTxPackages[blockHash]; ok {
		return txpkgs.Copy()
	}
	return types.TxPackages{}
}

func (bc *BlockChain) IsTxPackageInFuture(hash common.Hash) bool {
	bc.futureBlockTxPackagesMutex.RLock()
	defer bc.futureBlockTxPackagesMutex.RUnlock()
	for _, v := range bc.futureBlockTxPackages {
		if v.Has(hash) {
			return true
		}
	}
	return false
}

func (bc *BlockChain) GetRelatedBlockForFutureTxPackage(hash common.Hash) common.Hash {
	bc.futureBlockTxPackagesMutex.RLock()
	defer bc.futureBlockTxPackagesMutex.RUnlock()
	for k, v := range bc.futureBlockTxPackages {
		if v.Has(hash) {
			return k
		}
	}
	return common.Hash{}
}
