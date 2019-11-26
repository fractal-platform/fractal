// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package sync contains the implementation of fractal sync strategy.
// TODO: what will happen if malicious player exists
package sync

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	cursorNo = 1

	checkHeightMaxDiff = uint64(10)

	errMainBlockCheckAndExecFailed = errors.New("main block check or exec failed")
	errBlockCheckFailed            = errors.New("block check failed")
)

type execCache struct {
	blocks       types.Blocks
	hashes       []common.Hash
	mainChainSet mapset.Set
	hashIndex    map[common.Hash]int
}

// use cursor for block process
type Cursor struct {
	index        uint64 // index of hashList when exec blocks
	setHead      bool   //when sync blocks from checkpoint to fixPoint ,there is no need to checkGreedy and change head
	finished     int32
	running      int32  //atomic status indicate whether the cursor is running or not
	lowestHeight uint64 //if blockHeight<=lowestHeight+greedy ,no need to verify or exec

	execCache execCache

	chain  blockchain
	logger log.Logger

	//
	packer packer.Packer
}

func NewCursor(hashes []common.Hash, mainChainSet mapset.Set, chain blockchain, packer packer.Packer, lowestHeight uint64, setHead bool) *Cursor {
	hashIndex := make(map[common.Hash]int)
	for index, hash := range hashes {
		hashIndex[hash] = index
	}

	execCache := execCache{hashes: hashes, blocks: make(types.Blocks, len(hashes)), mainChainSet: mainChainSet, hashIndex: hashIndex}
	cursor := &Cursor{
		index:        0,
		chain:        chain,
		packer:       packer,
		lowestHeight: lowestHeight,
		execCache:    execCache,
		setHead:      setHead,
		logger:       log.NewSubLogger("m", fmt.Sprintf("cursor%d", cursorNo)),
	}
	cursorNo += 1
	return cursor
}

func (c *Cursor) Start() {
	atomic.StoreInt32(&c.running, 1)
}

func (c *Cursor) IsFinished() bool {
	return atomic.LoadInt32(&c.finished) == 1
}

func (c *Cursor) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

func (c *Cursor) Finish() {
	atomic.StoreInt32(&c.finished, 1)
	c.close()
}
func (c *Cursor) close() {
	atomic.StoreInt32(&c.running, 0)
}

func (c *Cursor) incIndex() {
	c.logger.Info("cursor index change", "index", c.index+1)
	c.index = c.index + 1
}

func (c *Cursor) getIndexHash() common.Hash {
	if c.index <= uint64(len(c.execCache.hashes)-1) {
		return c.execCache.hashes[c.index]
	}
	return common.Hash{}
}

func (c *Cursor) setCache(block *types.Block) {
	if index, ok := c.execCache.hashIndex[block.FullHash()]; ok {
		c.execCache.blocks[index] = block
	} else {
		c.logger.Warn("cursor block is not as expected", "blockHash", block.FullHash())
	}
}

func (c *Cursor) ProcessBlock(block *types.Block) error {
	c.logger.Info("Process block in cursor", "index", c.index, "blockHeight", block.Header.Height,
		"blockRound", block.Header.Round, "blockHash", block.FullHash(), "len(hashList)", len(c.execCache.hashes),
		"indexHash", c.getIndexHash())

	if block.FullHash() == c.chain.Genesis().FullHash() {
		c.logger.Info("genesis block, no need to process")
		return nil
	}

	noNeedToProcessHeight := c.lowestHeight
	if c.lowestHeight != 0 {
		noNeedToProcessHeight = c.lowestHeight + uint64(c.chain.GetChainConfig().Greedy)
	}

	//set cache
	c.setCache(block)

	hashesLen := len(c.execCache.hashes)

	if c.index >= uint64(hashesLen) {
		c.logger.Info("process block has already finished")
		return nil
	}

	// try to exec block in main-chain
	for c.execCache.blocks[c.index] != nil {
		b := c.execCache.blocks[c.index]

		if b.Header.Height <= noNeedToProcessHeight {
			c.logger.Info("block is lower than lowestHeight+greedy, no need to process", "blockHeight", b.Header.Height, "lowestHeight+greedy", noNeedToProcessHeight)
		} else {
			//verify
			c.logger.Info("verify and exec block",
				"execBlockHeight", b.Header.Height,
				"execBlockRound", b.Header.Round, "execHash", b.FullHash())

			_, _, _, _, err := c.chain.VerifyBlock(b, c.setHead)
			if err != nil {
				return errBlockCheckFailed
			}

			//if in side chain
			if !c.execCache.mainChainSet.Contains(b.FullHash()) {
				c.logger.Info("insert block not in main chain",
					"execBlockHeight", b.Header.Height,
					"execBlockRound", b.Header.Round, "execHash", b.FullHash())
				c.chain.InsertBlockNoCheck(b)
			} else {
				var err error
				if c.setHead {
					c.chain.InsertBlock(b)
				} else {
					err = c.chain.InsertPastBlock(b)
				}

				if err != nil {
					// TODO: what should we do if the peer is malicious
					c.logger.Info("insert past block failed", "err", err)
					break
				}
			}

		}

		c.incIndex()
		if c.index >= uint64(hashesLen) {
			break
		}
	}
	if c.index == uint64(hashesLen) {
		c.logger.Info("process block has finished")
		c.Finish()
	}

	return nil
}

// insert tx package
//func (c *Cursor) insertTxPackage(pkg *types.TxPackage) bool {
//	hash := pkg.Hash()
//
//	if c.chain.HasTxPackage(hash) {
//		return false
//	}
//
//	// Run the import on a new thread
//	log.Debug("Importing propagated tx package", "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash)
//
//	//
//	err := c.chain.VerifyTxPackage(pkg)
//	if err != nil {
//		// Something went very wrong, drop the peer
//
//		log.Error("verify Propagated tx package failed", "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash, "err", err)
//		if err == chain.ErrTxPackageRelatedBlockNotFound {
//			//pkg.ReceivedFrom.(*Peer).RequestOneBlock(pkg.BlockFullHash())
//			return false
//		}
//		return false
//	}
//
//	// Run the actual import
//	if err := c.packer.InsertRemoteTxPackage(pkg); err != nil {
//		log.Error("insert Propagated tx package into pool failed", "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash, "err", err)
//	}
//
//	return true
//}
