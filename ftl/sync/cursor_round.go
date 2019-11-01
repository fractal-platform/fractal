// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package sync contains the implementation of fractal sync strategy.
// TODO: what will happen if malicious player exists
package sync

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	cursorRoundNo = 1
)

// use cursor for block process
type CursorRound struct {
	index    uint64 // index of hashList when exec blocks
	setHead  bool   //when sync blocks from checkpoint to fixPoint ,there is no need to checkGreedy and change head
	finished int32
	running  int32 //atomic status indicate whether the cursor is running or not

	blocks    types.Blocks
	hashElems protocol.HashElems

	chain  blockchain
	logger log.Logger

	//
	packer packer.Packer
}

func NewCursorRound(hashElems protocol.HashElems, chain blockchain, packer packer.Packer, setHead bool, remainedLen int) *CursorRound {
	cursor := &CursorRound{
		index:     0,
		chain:     chain,
		packer:    packer,
		blocks:    make(types.Blocks, 0),
		hashElems: hashElems,
		setHead:   setHead,
		logger:    log.NewSubLogger("m", fmt.Sprintf("cursorRoundNo%d", cursorRoundNo)),
	}
	cursorRoundNo += 1
	return cursor
}

func (c *CursorRound) Start() {
	atomic.StoreInt32(&c.running, 1)
}

func (c *CursorRound) IsFinished() bool {
	return atomic.LoadInt32(&c.finished) == 1
}

func (c *CursorRound) IsRunning() bool {
	return atomic.LoadInt32(&c.running) == 1
}

func (c *CursorRound) Finish() {
	atomic.StoreInt32(&c.finished, 1)
	c.close()
}
func (c *CursorRound) close() {
	atomic.StoreInt32(&c.running, 0)
}

func (c *CursorRound) checkBlock(block *types.Block) error {
	hashFrom := c.hashElems[0]
	if block.Header.Height < hashFrom.Height && (hashFrom.Height-block.Header.Height) >= checkHeightMaxDiff {
		return errors.New("block too low")
	}
	if block.Header.Height < hashFrom.Height && (hashFrom.Height-block.Header.Height) < checkHeightMaxDiff {
		return nil
	}

	heightDiff := block.Header.Height - hashFrom.Height
	// check whether block height exceeds
	if heightDiff >= uint64(len(c.hashElems)) {
		return errors.New("block is too high")
	}
	return nil
}

func (c *CursorRound) incIndex() {
	c.index++
}

func (c *CursorRound) getIndexHashElem() protocol.HashElem {
	if c.index <= uint64(len(c.hashElems)-1) {
		return *c.hashElems[c.index]
	}
	return protocol.HashElem{}
}

func (c *CursorRound) ProcessBlock(block *types.Block) error {
	c.logger.Info("Process block in cursorRound", "index", c.index, "blockHeight", block.Header.Height,
		"blockRound", block.Header.Round, "blockHash", block.FullHash(), "len(hashList)", len(c.hashElems),
		"indexHashElem", c.getIndexHashElem())

	if block.FullHash() == c.chain.Genesis().FullHash() {
		c.logger.Info("genesis block, no need to process")
		return nil
	}

	err := c.checkBlock(block)
	if err != nil {
		c.logger.Error("Check block failed", "err", err)
		return err
	}

	// sort blocks
	c.blocks = append(c.blocks, block)
	c.blocks.SortByRoundHash()

	// try to insert blocks
	var blocks = make(types.Blocks, len(c.blocks))
	copy(blocks, c.blocks)
	for _, block := range c.blocks {
		if block.Header.Round > c.hashElems[c.index].Round {
			continue
		}

		dependHash, err := c.chain.VerifyBlockDepend(block)
		if err != nil {
			c.logger.Info("Verify block failed in cursor", "block", block.FullHash(), "dependHash", dependHash, "err", err)
			continue
		}

		_, _, _, _, err = c.chain.VerifyBlock(block, c.setHead)
		if err != nil {
			continue
		}

		// insert
		c.chain.InsertBlockNoCheck(block)
		blocks.Remove(block.FullHash())

		// exec if necessary
		if block.FullHash() == c.hashElems[c.index].Hash {
			nextHash := common.Hash{}
			if c.index+1 < uint64(len(c.hashElems)) {
				nextHash = c.hashElems[c.index+1].Hash
			}
			c.logger.Info("exec block in main-chain", "execBlockHeight", block.Header.Height,
				"execBlockRound", block.Header.Round, "execHash", block.FullHash(), "nextHash", nextHash)

			var err error
			if c.setHead {
				c.chain.InsertBlock(block)
			} else {
				err = c.chain.InsertPastBlock(block)
			}

			if err != nil {
				// TODO: what should we do if the peer is malicious
				break
			}
			c.incIndex()
			if c.index >= uint64(len(c.hashElems)) {
				break
			}
		}
	}
	c.blocks = blocks

	if c.index >= uint64(len(c.hashElems)) {
		c.logger.Info("process block has finished")
		c.Finish()
	}
	return nil
}

// insert tx package
//func (c *CursorRound) insertTxPackage(pkg *types.TxPackage) bool {
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
