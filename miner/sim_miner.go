// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package miner contains implementations for block mining strategy.
package miner

import (
	"math/big"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto/sha3"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// simMiner creates blocks for simulations.
type simMiner struct {
	newMinedBlockFeed *event.Feed
	quitCh            chan interface{}
	isMining          bool

	coinbase   common.Address
	difficulty *big.Int
	greedy     uint8

	chain blockChain

	amount        uint64
	height        uint64
	total_parents uint64
	rounds        uint64
	useHeight     bool

	// Subscriptions
	chainUpdateCh  chan types.ChainUpdateEvent
	chainUpdateSub event.Subscription
}

func NewSimMiner(difficulty *big.Int, greedy uint8, rounds uint64, useHeight bool, blockChain blockChain) Miner {
	miner := &simMiner{
		newMinedBlockFeed: new(event.Feed),
		quitCh:            make(chan interface{}),
		isMining:          false,
		difficulty:        difficulty,
		greedy:            greedy,
		chain:             blockChain,
		amount:            0,
		height:            0,
		total_parents:     0,
		rounds:            rounds,
		useHeight:         useHeight,
	}

	// Subscribe events for blockchain
	miner.chainUpdateSub = miner.chain.SubscribeChainUpdateEvent(miner.chainUpdateCh)

	return miner
}

func (self *simMiner) Start() {
	self.isMining = true
	self.loop()
}

func (self *simMiner) Stop() {
	if self.isMining {
		self.isMining = false
		close(self.quitCh)
	}
}

func (self *simMiner) Close() {}

func (self *simMiner) IsMining() bool {
	return self.isMining
}

func (self *simMiner) GetCoinbase() common.Address {
	return self.coinbase
}

func (self *simMiner) SetCoinbase(addr common.Address) {
	self.coinbase = addr
}

func (self *simMiner) SubscribeNewMinedBlockEvent(ch chan<- types.NewMinedBlockEvent) event.Subscription {
	return self.newMinedBlockFeed.Subscribe(ch)
}

func (self *simMiner) loop() {
	target := new(big.Int).Div(maxUint256, self.difficulty)
	currentRound := uint64(time.Now().UnixNano())
	for i := uint64(0); i < self.rounds; i++ {
		var parents types.Blocks
		if self.useHeight {
			//parents = self.chain.GetGreedyBlocks2(self.greedy)
		} else {
			parents = self.chain.GetGreedyBlocks(self.greedy)
		}
		self.total_parents += uint64(len(parents))
		log.Warn("set parents", "blocks", len(parents), "amount", self.amount, "height", self.height, "width", float64(self.total_parents)/float64(self.amount))

		var hash common.Hash
		currentRound = currentRound + i
		for _, parent := range parents {
			hw := sha3.NewKeccak256()
			rlp.Encode(hw, []interface{}{
				parent.FullHash(),
				currentRound,
				self.coinbase,
			})
			hw.Sum(hash[:0])
			if new(big.Int).SetBytes(hash.Bytes()).Cmp(target) <= 0 {
				block := self.generateBlock(currentRound, parent)
				//self.chain.InsertBlockNoCheck(block)

				//currentBlock := self.chain.CurrentBlock()
				//if block.Header.Height > currentBlock.Header.Height ||
				//	(block.Header.Height == currentBlock.Header.Height &&
				//		block.FullHash().Big().Cmp(currentBlock.FullHash().Big()) < 0) {
				//	self.chain.SetCurrentBlock(block)
				//}

				self.amount += 1
				if block.Header.Height > self.height {
					self.height = block.Header.Height
				}
			}
		}
	}
}

func (self *simMiner) generateBlock(round uint64, parent *types.Block) *types.Block {
	block := types.NewBlock(parent.SimpleHash(), round, []byte{}, self.coinbase, self.difficulty, parent.Header.Height+1)
	block.Header.ParentFullHash = parent.FullHash()
	log.Info("generate block", "round", round, "height", block.Header.Height, "parent", parent.FullHash(), "hash", block.FullHash())
	return block
}
