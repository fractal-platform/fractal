// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync fixpoint contains the implementation of fractal sync fixpoint.

package sync

import (
	"errors"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"time"
)

/**
 	sync and check states and Blocks around one fixPoint
	call parallel Blocks sync  from HashEFrom to HashETo, and bestHashE is head block of bestPeer
	doSyncAndCheckFixPoint is in charge of register/unRegister peers in downLoader

	peers []peer : where we sync states and Blocks
	fixpoint HashElem: fixPoint
	hashETo HashElem:  we can sync Blocks concurrently from hashEFrom to hashETo
	bestHashE HashElem: there is a little diff in head Blocks,so we can't sync concurrently
	blockSyncHashList HashElems: for concurrent sync Blocks to identify where we sync that moment
	setHead bool: when we sync checkpoint ,we can't set current head,because miner should already start

	err: errPeer (wrong with peer) ,forceQuit(quit initiative)
 */
func (s *Synchronizer) doSyncAndCheckFixPoint(peers []peer, bestPeer peer, commonHashElems protocol.HashElems, bestHashElem protocol.HashElem, setHead bool) (bool, []peer, error) {
	fixpoint := commonHashElems[0]
	commonHighestHashElem := commonHashElems[len(commonHashElems)-1]
	s.log.Info("start to sync and check fix point", "bestPeer", bestPeer, "peers", peers,
		"fixpoint", fixpoint, "commonHighestHashElem", commonHighestHashElem, "bestHash", bestHashElem, "commonHashElems", len(commonHashElems), "setHead", setHead)

	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointPreBlocks)

	var peerMap = make(map[string]downloader.FetcherPeer)
	for _, peer := range peers {
		peerMap[peer.GetID()] = peer
	}

	var preSyncLength int
	if fixpoint.Height >= uint64(s.lengthForStatesSync()) {
		preSyncLength = s.lengthForStatesSync()
	} else {
		preSyncLength = int(fixpoint.Height)
	}

	// request for pre block and states
	if fixpoint.Hash == s.chain.Genesis().FullHash() {
		s.log.Info("hashFrom is genesis, no need to sync pre blocks and states")
	} else {
		err := bestPeer.RequestSyncPreBlocksForState(fixpoint.Hash)
		if err != nil {
			s.log.Error("request pre blocks failed", "err", err)
			return false, []peer{bestPeer}, errPeer
		}

		// process pre block
		var fixPointBlock *types.Block
		var blocks types.Blocks
		timeout := time.NewTimer(time.Duration(s.config.TimeOutOfFixPointPreBlock) * time.Microsecond)
		select {
		case <-timeout.C:
			s.log.Error("request pre blocks timeout")
			return false, []peer{bestPeer}, errPeer
		case blocks = <-s.blocksForPreStateRevCh:
			s.log.Info("received pre blocks", "len", len(blocks))
			if nil == blocks || (len(blocks) < s.lengthForStatesSync() && fixpoint.Height >= uint64(s.lengthForStatesSync())-1) {
				s.log.Error("sync pre blocks failed, block length too small")
				return false, []peer{bestPeer}, errPeer
			}

			fixPointBlock = blocks[0]
			var blockMap = make(map[common.Hash]*types.Block)
			s.log.Info("fix point info", "height", fixPointBlock.Header.Height, "hash", fixPointBlock.FullHash(), "round", fixPointBlock.Header.Round)
			for _, block := range blocks {
				s.chain.InsertBlockNoCheck(block)
				blockMap[block.FullHash()] = block
			}

		}
		if setHead {
			s.lastHeadBlock = s.chain.CurrentBlock()
			s.chain.SetCurrentBlock(fixPointBlock)
		}

		// set status for fast sync
		s.changeFastSyncStatus(FastSyncStatusFixPointPreStates)

		// state sync
		s.log.Info("request for state sync", "hash", fixPointBlock.FullHash(), "round", fixPointBlock.Header.Round, "Height", fixPointBlock.Header.Height, "len(states)", preSyncLength)
		for i := preSyncLength - 1; i >= 0; i-- {
			// todo: blocks should be sorted first
			root := blocks[i].Header.StateHash
			s.log.Info("start sync state", "rootHash", root, "Height", blocks[i].Header.Height, "fullHash", blocks[i].FullHash())

			var peersErr = make([]peer, 10)
			s.stateSync = downloader.SyncState(peerMap, func(id string, addBlack bool) {
				peersErr = append(peersErr, peerMap[id].(peer))
				delete(peerMap, id)
				s.removePeerCallback(id, addBlack)
			}, root, s.chain.Database())
			err := s.stateSync.Wait()
			s.stateSync = nil
			if err != nil {
				s.log.Error("sync state failed", "error", err)
				//return peers wrong
				return false, peersErr, errPeer
			}

			// set state flag as if it is not executed
			dbaccessor.WriteBlockStateCheck(s.chain.Database(), blocks[i].FullHash(), types.HasBlockStateButNotChecked)

			select {
			case <-s.fastSyncQuitCh:
				s.log.Info("RequestBlocksForPostStateSync from peer fastSyncQuitCh force quit")
				return false, nil, errors.New("sync recv quit")
			default:
			}
		}
	}

	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointPostBlocks)

	// request for post block
	s.log.Info("sync post blocks", "hashElemFrom", fixpoint, "hashElemTo", commonHighestHashElem, "bestHashElem", bestHashElem)
	if fixpoint != commonHighestHashElem {
		var peersErr []peer
		var blockCh = make(chan *types.Block)
		s.blockSync = downloader.StartFetchBlocks(fixpoint.Round-1, commonHighestHashElem.Round, peerMap, func(id string, addBlack bool) {
			peersErr = append(peersErr, peerMap[id].(peer))
			s.removePeerCallback(id, addBlack)
		}, true, protocol.SyncStageFastSync, s.chain, blockCh)

		quitCh := make(chan struct{})
		go func() {
			cursor := NewCursor(commonHashElems, s.chain, s.packer, true, s.lengthForStatesSync())
		ForLoop:
			for {
				select {
				case block := <-blockCh:
					err := cursor.ProcessBlock(block)
					if err == ErrMainBlockCheckAndExecFailed {
						break ForLoop
					}
					if cursor.IsFinished() {
						break ForLoop
					}
				case <-quitCh:
					return
				}
			}
			s.blockSync.Finish()
		}()

		err := s.blockSync.Wait()
		s.blockSync = nil
		close(quitCh)
		//close(blockCh)
		if err != nil {
			s.log.Error("syncBlock for post failed", "hashFrom", fixpoint, "hashTo", commonHighestHashElem)
			return false, peersErr, errPeer
		}
		s.log.Info("Blocks received and executed")
	}
	// if bestHashE is not nil ,call best peer to sync post Blocks of very high
	if bestHashElem != (protocol.HashElem{}) && bestHashElem.String() != commonHighestHashElem.String() {
		s.log.Info("request higher post blocks", "hashTo", commonHighestHashElem, "bestHashElem", bestHashElem)
		err := bestPeer.RequestSyncPostBlocksForState(*commonHighestHashElem, bestHashElem)
		if err != nil {
			s.log.Error("request higher post blocks failed", "peer", bestPeer.Name(), "bestHash", bestHashElem, "err", err)
			return false, []peer{bestPeer}, errPeer
		}
		for {
			var finished = false
			timeout := time.NewTimer(time.Duration(s.config.ShortTimeOutOfSyncVeryHigh) * time.Microsecond)
			select {
			case <-timeout.C:
				s.log.Error("RequestPostBlocksForStateSync from peer timeout", "peer", bestPeer.Name(), "hashTo", commonHighestHashElem, "bestHash", bestHashElem, "err", "timeout")
				return false, []peer{bestPeer}, errPeer
			case blocks := <-s.blocksForPostStateRevCh:
				// finish and exec
				if len(blocks) <= 0 {
					finished = true
				} else {
					s.log.Info("RequestPostBlocksForStateSync", "len(Blocks)", len(blocks), "Blocks", blocks)
					for _, block := range blocks {
						s.blockProcessCh <- &network.BlockWithVerifyFlag{Block: block, Verify: true}
					}
				}
			case <-s.fastSyncQuitCh:
				s.log.Info("fastSync force quit")
				return false, nil, errors.New("fastSync force quit")
			}
			if finished {
				break
			}
		}
	}

	// wait for block process finish
	s.log.Info("wait for higher post blocks process finish")
	timeout := time.NewTimer(time.Duration(s.config.LongTimeOutOfFixPointFinish) * time.Microsecond)
	for {
		if bestHashElem != (protocol.HashElem{}) && bestHashElem.CompareTo(*commonHighestHashElem) > 0 {
			if s.chain.HasBlock(bestHashElem.Hash) {
				s.log.Info("RequestBlocksForStateSync bestHashE last block processed")
				break
			}
		} else {
			if s.chain.HasBlock(commonHighestHashElem.Hash) {
				s.log.Info("RequestBlocksForStateSync hashETo last block processed")
				break
			}
		}

		//consume quitCh
		select {
		case <-timeout.C:
			s.log.Error("RequestBlocksForPostStateSync from peer wait block process timeout", "peer", bestPeer.Name(), "hashElem", commonHighestHashElem, "bestHash", bestHashElem)
			if bestHashElem != (protocol.HashElem{}) && bestHashElem.String() != commonHighestHashElem.String() {
				return false, []peer{bestPeer}, errPeer
			} else {
				return false, peers, errPeer
			}

		case <-s.fastSyncQuitCh:
			s.log.Info("RequestBlocksForPostStateSync from peer fastSyncQuitCh force quit")
			return false, nil, errors.New("sync recv quit")
		default:
		}
		time.Sleep(1 * time.Second)
	}
	s.log.Info("sync fix point finished")
	return true, nil, nil
}
