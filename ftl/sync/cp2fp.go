// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync cp2fp contains the implementation of fractal sync checkpoint to fixpoint.
package sync

import (
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
)

type CP2FPSync struct {
	task     *CP2FPTask
	taskLock sync.RWMutex
	taskCh   chan *CP2FPTask

	// for skeleton hash process
	peerHashTreeRspCh chan protocol.SyncHashTreeRsp

	sync         *Synchronizer
	removePeerFn removePeerCallback
	logger       log.Logger
}

func newCP2FPSync(peerHashTreeRspCh chan protocol.SyncHashTreeRsp, sync *Synchronizer) *CP2FPSync {
	res := &CP2FPSync{
		task:   nil,
		taskCh: make(chan *CP2FPTask),

		peerHashTreeRspCh: peerHashTreeRspCh,
		sync:              sync,
		removePeerFn:      sync.removePeerCallback,
		logger:            sync.log,
	}
	go res.loop()
	return res
}

func (s *CP2FPSync) loop() {
	for task := range s.taskCh {
		s.taskLock.Lock()
		s.task = task
		s.taskLock.Unlock()

		s.task.process()

		s.taskLock.Lock()
		s.task = nil
		s.taskLock.Unlock()
	}
}

func (s *CP2FPSync) startTask(currentHeight uint64, hashTo common.Hash, accHash common.Hash, peers []peer) {
	// stop first
	s.stopAll()

	// find latest check point
	checkPoint, _ := s.sync.chain.GetLatestCheckPointBelowHeight(currentHeight, true)
	s.logger.Info("cp2fp task", "checkPoint", checkPoint.TreePoint)
	if (currentHeight - checkPoint.Height) < types.HashTreeMinLength {
		s.logger.Info("local hash tree length is lower than min hash tree length, no need to sync", "currentHeight", currentHeight, "checkPointHeight", checkPoint.Height, "minHashTreeLength", types.HashTreeMinLength)
		s.sync.chain.StartCreateCheckPoint()
		return
	}

	// check if no need to sync
	_, _, err := s.sync.chain.CreateHashTree(checkPoint.FullHash, hashTo)
	if err == nil {
		s.logger.Info("local hash tree is full, no need to sync")
		s.sync.chain.StartCreateCheckPoint()
		return
	}

	task := &CP2FPTask{
		quitCh: make(chan struct{}),

		from:    checkPoint.FullHash,
		to:      hashTo,
		accHash: accHash,
		peers:   peers,


		peerHashTreeRspCh: s.peerHashTreeRspCh,

		sync:         s.sync,
		removePeerFn: s.removePeerFn,
		logger:       s.logger,
	}
	s.logger.Info("start cp2fp task", "fromHash", checkPoint.FullHash, "fromHeight", checkPoint.Height, "toHash", hashTo, "accHash", accHash, "honestPeers", peers)
	s.taskCh <- task
}

func (s *CP2FPSync) isRunning() bool {
	s.taskLock.RLock()
	defer s.taskLock.RUnlock()
	return s.task != nil
}

func (s *CP2FPSync) stopAll() {
	s.taskLock.Lock()
	if s.task != nil {
		s.task.stop()
		s.task = nil
	}
	s.taskLock.Unlock()
}

func (s *CP2FPSync) registerPeer(p peer) {
	s.taskLock.RLock()
	if s.task != nil {
		if s.task.blockSync != nil {
			s.task.blockSync.Register(p)
		}
	}
	s.taskLock.RUnlock()
}

func (s *CP2FPSync) deliverData(id string, data interface{}, kind int) error {
	if s.task != nil {
		if s.task.blockSync != nil {
			return s.task.blockSync.DeliverData(id, data, kind)
		}
	}
	return nil
}

type CP2FPTask struct {
	quitCh   chan struct{}
	quitOnce sync.Once

	// task info
	from    common.Hash
	to      common.Hash
	accHash common.Hash
	peers   []peer

	// for block sync
	blockSync *downloader.BlockFetcherByRound

	// for skeleton hash process
	peerHashTreeRspCh chan protocol.SyncHashTreeRsp

	sync         *Synchronizer
	removePeerFn removePeerCallback
	logger       log.Logger
}

func (t *CP2FPTask) process() {
	from := t.from
	to := t.to
	accHash := t.accHash
	peers := t.peers

	t.logger.Info("start fulfill from point to point", "from", from, "to", to, "peers", len(peers))

	for len(peers) > 0 {
		bestPeer := getBestPeerByHead(peers)
		errPeers, err := t.sync.syncTreeBlockAndStates(peers, bestPeer, from, to, accHash, t.sync.config.LongTimeOutOfCp2fpHashTree, false, true)

		if err != nil {
			//delete the peer and do it again
			for _, ePeer := range errPeers {
				for i, peer := range peers {
					if peer.GetID() == ePeer.GetID() {
						peers = append(peers[0:i], peers[i+1:]...)
						break
					}
				}
			}
			t.logger.Error("fulfill from point to point failed", "err", err, "errPeer", errPeers)
			continue
		}
		t.logger.Info("check point to fix point blocks received and executed", "blockFrom", from, "blockTo", to)
		t.sync.chain.StartCreateCheckPoint()
		return
	}
}

func (t *CP2FPTask) stop() {
	t.logger.Info("force stop cp2fp tasks")
	t.quitOnce.Do(func() {
		close(t.quitCh)
	})
}
