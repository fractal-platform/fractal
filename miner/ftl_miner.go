// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package miner contains implementations for block mining strategy.
package miner

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

type blockChain interface {
	GetTxPackageList(hashes []common.Hash) types.TxPackages

	HasBlock(hash common.Hash) bool
	GetBlock(hash common.Hash) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
	GetStateBeforeCacheHeight(block *types.Block, cacheHeight uint8) (*state.StateDB, *types.Block, bool)
	GetPreBalanceAndPubkey(block *types.Block, address common.Address) (uint64, []byte, error)
	GetGreedyBlocks(greedy uint8) types.Blocks
	GetBlocksFromBlockRange(b1 *types.Block, b2 *types.Block) types.Blocks // (round1, round2], sorted by round & hash
	InsertBlockWithState(block *types.Block, state *state.StateDB, receipts types.Receipts, executedTxs []*types.TxWithIndex, bloom *types.Bloom)
	CalcAndCheckState(block *types.Block) bool
	SubscribeChainUpdateEvent(ch chan<- types.ChainUpdateEvent) event.Subscription
	GetHopCount(block1 *types.Block, block2 *types.Block) (uint64, error)
	Database() dbwrapper.Database
	GetChainID() uint64
	GetGreedy() uint8
	GetChainConfig() *config.ChainConfig
	CheckGreedy(block1 *types.Block, block2 *types.Block, greedy uint64) (bool, error)
	ValidatePackage(pkg *types.TxPackage, height uint64) error
}

// ftlMiner creates blocks and searches for proof-of-stake values.
type ftlMiner struct {
	worker   *worker
	coinbase common.Address

	newMinedBlockFeed *event.Feed
	blockChain        blockChain
}

func NewFtlMiner(blockChain blockChain, executor txexec.TxExecutor, txPool pool.Pool, pkgPool pool.Pool, keyman *keys.MiningKeyManager) Miner {
	miner := &ftlMiner{
		newMinedBlockFeed: new(event.Feed),
		blockChain:        blockChain,
	}
	miner.worker = newWorker(blockChain, executor, txPool, pkgPool, miner.newMinedBlockFeed, keyman)

	return miner
}

func (self *ftlMiner) Start() {
	log.Info("start miner")
	self.worker.start()
}

func (self *ftlMiner) Stop() {
	log.Info("stop miner")
	self.worker.stop()
}

func (self *ftlMiner) Close() {
	log.Info("close miner")
	self.worker.close()
}

func (self *ftlMiner) IsMining() bool {
	return self.worker.isRunning()
}

func (self *ftlMiner) GetCoinbase() common.Address {
	return self.coinbase
}

func (self *ftlMiner) SetCoinbase(addr common.Address) {
	self.coinbase = addr
	self.worker.setCoinbase(addr)
}

func (self *ftlMiner) SubscribeNewMinedBlockEvent(ch chan<- types.NewMinedBlockEvent) event.Subscription {
	return self.newMinedBlockFeed.Subscribe(ch)
}
