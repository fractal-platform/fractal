// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package network contains the implementation of network protocol handler for fractal.
package network

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"sync"
	"time"
)

type taskType int

const (
	taskTypeBegin = iota
	taskTypePropagateBlock
	taskTypeAnnounceTxPkg
	taskTypePropagateTxPkg
	taskTypePropagateTx
	taskTypeEnd
)

const (
	// maxQueuedBlockProps is the maximum number of block propagation to queue up before dropping broadcasts.
	maxQueuedBlockProps = 128

	// maxQueuedTxPackageProps is the maximum number of tx packages to queue up before dropping broadcasts.
	maxQueuedTxPkgProps = 1024

	// maxQueuedTxPackageAnns is the maximum number of tx package announcements to queue up before dropping broadcasts.
	maxQueuedTxPkgAnns = 1024

	// maxQueuedTxs is the maximum number of transaction lists to queue up before
	// dropping broadcasts. This is a sensitive number as a transaction list might
	// contain a single transaction, or thousands.
	maxQueuedTxs = 1024
)

var taskChannelSizeMap map[taskType]int

func init() {
	taskChannelSizeMap = make(map[taskType]int)
	taskChannelSizeMap[taskTypePropagateBlock] = maxQueuedBlockProps
	taskChannelSizeMap[taskTypeAnnounceTxPkg] = maxQueuedTxPkgProps
	taskChannelSizeMap[taskTypePropagateTxPkg] = maxQueuedTxPkgAnns
	taskChannelSizeMap[taskTypePropagateTx] = maxQueuedTxs
}

type taskPipe struct {
	channels map[taskType]chan interface{}
	notify   chan struct{}

	// for counter
	counts map[taskType]int
	mutex  sync.Mutex
}

func newTaskPipe() *taskPipe {
	tp := &taskPipe{
		channels: make(map[taskType]chan interface{}),
		counts:   make(map[taskType]int),
	}

	// init channel for each task type
	var i taskType
	var taskChannelTotalSize = 0
	for i = taskTypeBegin + 1; i < taskTypeEnd; i++ {
		tp.channels[i] = make(chan interface{}, taskChannelSizeMap[i])
		tp.counts[i] = 0
		taskChannelTotalSize += taskChannelSizeMap[i]
	}

	// init notify channel
	tp.notify = make(chan struct{}, taskChannelTotalSize)

	return tp
}

// loop is a write loop that multiplexes messages into the remote peer.
// The goal is to have an async writer that does not lock up node internals.
func (p *Peer) loop() {
	for {
		select {
		case <-p.pipe.notify:
			// process one task for each notify
			var i taskType
			for i = taskTypeBegin + 1; i < taskTypeEnd; i++ {
				if p.processChannel(i, p.pipe.channels[i]) {
					break
				}
			}

		case <-p.term:
			return
		}
	}
}

// process one task from channel
func (p *Peer) processChannel(taskType taskType, channel chan interface{}) bool {
	select {
	case d := <-channel:
		p.processTask(taskType, d)
		return true
	default:
		return false
	}
}

func (p *Peer) increaseCount(taskType taskType) {
	p.pipe.mutex.Lock()
	p.pipe.counts[taskType] += 1
	p.pipe.mutex.Unlock()
}

func (p *Peer) processTask(taskType taskType, taskData interface{}) {
	var count int
	p.pipe.mutex.Lock()
	p.pipe.counts[taskType] -= 1
	count = p.pipe.counts[taskType]
	p.pipe.mutex.Unlock()

	switch taskType {
	case taskTypePropagateBlock:
		block := taskData.(*types.Block)
		p.Log().Info("Propagate block", "hash", block.FullHash(), "pipe", count, "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
		if err := p.SendNewBlock(block); err != nil {
			p.Log().Error("Propagate block failed", "err", err)
			return
		}
		p.Log().Info("Propagate block OK", "hash", block.FullHash(), "pipe", count, "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
	case taskTypeAnnounceTxPkg:
		pkgHash := taskData.(common.Hash)
		//p.Log().Info("Announce tx package", "hash", pkgHash, "pipe", count)
		if err := p.SendTxPackageHash(pkgHash); err != nil {
			p.Log().Error("Announce tx package failed", "err", err)
			return
		}
	case taskTypePropagateTxPkg:
		pkg := taskData.(*types.TxPackage)
		//p.Log().Info("Propagate tx package", "hash", pkg.Hash(), "pipe", count, "duration", common.PrettyDuration(time.Since(pkg.ReceivedAt)))
		if err := p.SendTxPackage(pkg); err != nil {
			p.Log().Error("Propagate tx package failed", "err", err)
			return
		}
		//p.Log().Info("Propagate tx package OK", "hash", pkg.Hash(), "pipe", count, "duration", common.PrettyDuration(time.Since(pkg.ReceivedAt)))
	case taskTypePropagateTx:
		txs := taskData.(types.Transactions)
		p.Log().Info("Propagate transactions", "txCount", len(txs), "pipe", count)
		if err := p.SendTransactions(txs); err != nil {
			p.Log().Error("Propagate transactions failed", "err", err)
			return
		}
	}
}

// AsyncSendNewBlock queues an entire block for propagation to a remote peer. If
// the peer's broadcast queue is full, the event is silently dropped.
func (p *Peer) AsyncSendNewBlock(block *types.Block) {
	select {
	case p.pipe.channels[taskTypePropagateBlock] <- block:
		p.knownBlocks.Add(block.FullHash())
		p.increaseCount(taskTypePropagateBlock)
		p.pipe.notify <- struct{}{}
	default:
		p.Log().Warn("Dropping block propagation", "Height", block.Header.Height, "Hash", block.FullHash())
	}
}

// AsyncSendTxPackageHash queues the availability of tx package for propagation to a
// remote peer. If the peer's broadcast queue is full, the event is silently
// dropped.
func (p *Peer) AsyncSendTxPackageHash(pkg *types.TxPackage) {
	select {
	case p.pipe.channels[taskTypeAnnounceTxPkg] <- pkg.Hash():
		p.knownTxPackages.Add(pkg.Hash())
		p.increaseCount(taskTypeAnnounceTxPkg)
		p.pipe.notify <- struct{}{}
	default:
		p.Log().Warn("Dropping tx package announcement", "Hash", pkg.Hash())
	}
}

// AsyncSendTxPackage queues tx package propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *Peer) AsyncSendTxPackage(pkg *types.TxPackage) {
	select {
	case p.pipe.channels[taskTypePropagateTxPkg] <- pkg:
		p.knownTxPackages.Add(pkg.Hash())
		p.increaseCount(taskTypePropagateTxPkg)
		p.pipe.notify <- struct{}{}
	default:
		p.Log().Warn("Dropping tx package propagation", "Hash", pkg.Hash())
	}
}

// AsyncSendTransactions queues list of transactions propagation to a remote
// peer. If the peer's broadcast queue is full, the event is silently dropped.
func (p *Peer) AsyncSendTransactions(txs types.Transactions) {
	select {
	case p.pipe.channels[taskTypePropagateTx] <- txs:
		for _, tx := range txs {
			p.knownTxs.Add(tx.Hash())
		}
		p.increaseCount(taskTypePropagateTx)
		p.pipe.notify <- struct{}{}
	default:
		p.Log().Warn("Dropping transaction propagation", "count", len(txs))
	}
}
