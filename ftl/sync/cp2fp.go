// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync cp2fp contains the implementation of fractal sync checkpoint to fixpoint.
package sync

import (
	"errors"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
	"sync"
)

const (
	taskBackTrackLength = 10
)

type CP2FPSync struct {
	task     *CP2FPTask
	taskLock sync.RWMutex

	// for skeleton hash process
	peerSkeletonCh  chan PeerHashElemList
	timeoutSkeleton int

	sync         *Synchronizer
	removePeerFn removePeerCallback
	logger       log.Logger
}

func newCP2FPSync(peerSkeletonCh chan PeerHashElemList, timeoutSkeleton int, sync *Synchronizer) *CP2FPSync {
	res := &CP2FPSync{
		task: nil,

		peerSkeletonCh:  peerSkeletonCh,
		timeoutSkeleton: timeoutSkeleton,
		sync:            sync,
		removePeerFn:    sync.removePeerCallback,
		logger:          sync.log,
	}
	return res
}

func (s *CP2FPSync) startTask(blockFrom *types.Block, blockTo *types.Block, peers []peer) {
	// stop first
	s.stopAll()

	// find break point
	var fromHashElem protocol.HashElem
	var toHashElem protocol.HashElem
	fromBlock, toBlock, err := s.sync.chain.GetBreakPoint(blockFrom, blockTo)
	if err != nil {
		s.logger.Error("can't find from or to break point", "err", err)
		return
	} else if fromBlock == nil || toBlock == nil {
		s.logger.Info("break point is nil, no need to sync")
		return
	} else {
		for i := 0; i < taskBackTrackLength; i++ {
			oldFromBlock := fromBlock
			fromBlock = s.sync.chain.GetBlock(fromBlock.Header.ParentFullHash)
			if fromBlock == nil {
				fromBlock = oldFromBlock
				break
			}
		}

		fromHashElem = protocol.HashElem{Height: fromBlock.Header.Height, Hash: fromBlock.FullHash(), Round: fromBlock.Header.Round}
		toHashElem = protocol.HashElem{Height: toBlock.Header.Height, Hash: toBlock.FullHash(), Round: toBlock.Header.Round}
	}

	s.task = &CP2FPTask{
		quitCh: make(chan struct{}),

		from:  fromHashElem,
		to:    toHashElem,
		peers: peers,

		peerSkeletonCh:  s.peerSkeletonCh,
		timeoutSkeleton: s.timeoutSkeleton,

		sync:         s.sync,
		removePeerFn: s.removePeerFn,
		logger:       s.logger,
	}
	s.logger.Info("start cp2fp task", "fromHashElem", fromHashElem, "toHashElem", toHashElem, "honestPeers", peers)
	go s.task.process()
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
	from  protocol.HashElem
	to    protocol.HashElem
	peers []peer

	// for block sync
	blockSync *downloader.BlockFetcher

	// for skeleton hash process
	peerSkeletonCh  chan PeerHashElemList
	timeoutSkeleton int

	sync         *Synchronizer
	removePeerFn removePeerCallback
	logger       log.Logger
}

func (t *CP2FPTask) process() {
	from := t.from
	to := t.to
	peers := t.peers

	//if to is below from, no need to sync
	if (from.Height == 0 && to.Height <= from.Height+2) || (from.Height != 0 && to.Height <= from.Height) {
		t.logger.Info("no need to sync blocks", "from.Height", from.Height, "to.Height", to.Height)
		return
	}
	t.logger.Info("start fulfill from point to point", "from", from, "to", to, "peers", len(peers))

	for len(peers) > 0 {
		var longHashList protocol.HashElems
		bestPeer := getBestPeerByHead(peers)

		// invoke hash fetcher
		length := int(to.Height - from.Height + 1)

		fetcher := newLongHashFetcher([]peer{bestPeer}, protocol.SyncStageCP2FP, length, from, to, t.peerSkeletonCh, t.timeoutSkeleton, t.removePeerFn, t.logger)
		err := fetcher.fetch()
		if err != nil {
			//delete the peer and do it again
			for i, peer := range peers {
				if peer.GetID() == bestPeer.GetID() {
					peers = append(peers[0:i], peers[i+1:]...)
					break
				}
			}
			t.logger.Error("sync long list for full fill failed", "err", err, "peer", bestPeer.GetID())
			continue
		}

		// get long hash list
		longListMap := fetcher.fetchResult.hashes
		longHashList, ok := longListMap[bestPeer.GetID()]
		if !ok {
			t.logger.Error("sync long list for full fill failed", "err", errors.New("can't find long hash list"), "peer", bestPeer.GetID())
			continue
		}

		t.logger.Info("fetch long hash list ok", "longHashListSize", len(longHashList))
		//TODO: fork branch
		// revert long hash list
		var longHashListReverse protocol.HashElems
		for i := len(longHashList) - 1; i >= 0; i-- {
			longHashListReverse = append(longHashListReverse, longHashList[i])
		}

		var peerMap = make(map[string]downloader.FetcherPeer)
		for _, peer := range t.sync.getPeers() {
			peerMap[peer.GetID()] = peer
		}

		//remove genesis hash
		if longHashListReverse[0].Hash == t.sync.chain.Genesis().FullHash() {
			t.logger.Info("remove genesis from long list", "genesis", longHashListReverse[0])
			longHashListReverse = longHashListReverse[1:]
			from = *longHashListReverse[0]
		}

		t.logger.Info("getBlocksFromCheckpointToFixPoint", "len(longHashListReverse)", len(longHashListReverse), "hashFrom", longHashListReverse[0],
			"hashTo", longHashListReverse[len(longHashListReverse)-1], "allPeersForDownloader", peerMap, "genesisRound", t.sync.chain.Genesis().Header.Round)
		var blockCh = make(chan *types.Block)
		t.blockSync = downloader.StartFetchBlocks(from.Round-1, to.Round, peerMap, func(id string, addBlack bool) {
			t.removePeerFn(id, addBlack)
		}, false, protocol.SyncStageCP2FP, t.sync.chain, blockCh)

		cursor := NewCursor(longHashListReverse, t.sync.chain, t.sync.packer, false, t.sync.lengthForStatesSync())
	ForLoop:
		for {
			select {
			case block := <-blockCh:
				err := cursor.ProcessBlock(block)
				if err == ErrMainBlockCheckAndExecFailed {
					log.Error("main hash list is wrong, it is impossible")
				}
				if cursor.IsFinished() {
					break ForLoop
				}
			case <-t.quitCh:
				t.logger.Info("cp2fp task stopCh force closed")
				break ForLoop
			}
		}
		t.blockSync.Finish()
		t.blockSync = nil
		t.logger.Info("check point to fix point blocks received and executed", "blockFrom", from, "blockTo", to)
		return
	}
}

func (t *CP2FPTask) stop() {
	t.logger.Info("force stop cp2fp tasks")
	t.quitOnce.Do(func() {
		close(t.quitCh)
	})
}
