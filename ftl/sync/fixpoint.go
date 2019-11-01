// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync fixpoint contains the implementation of fractal sync fixpoint.

package sync

import (
	"errors"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
)

const (
	traceBackLengthForBestPeerBlocks = 10
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
	fixPoint := commonHashElems[0]
	commonHighestHashElem := commonHashElems[len(commonHashElems)-1]
	traceBackBestPeerHashElem := commonHashElems[len(commonHashElems)-1-traceBackLengthForBestPeerBlocks]

	s.log.Info("start to sync and check fix point", "bestPeer", bestPeer, "peers", peers,
		"fixPoint", fixPoint, "commonHighestHashElem", commonHighestHashElem, "bestHash", bestHashElem, "commonHashElems", len(commonHashElems), "setHead", setHead)

	accHash := commonHighestHashElem.AccHash

	errPeers, err := s.syncTreeBlockAndStates(peers, bestPeer, fixPoint.Hash, commonHighestHashElem.Hash, accHash, s.config.TimeOutOfFixPointHashTree, setHead, false)
	if err != nil {
		return false, errPeers, err
	}

	if bestHashElem != (protocol.HashElem{}) && bestHashElem.Hash != commonHighestHashElem.Hash {
		// sync best peer blocks
		errPeers, err = s.syncBestBlocks(bestHashElem, *traceBackBestPeerHashElem, bestPeer)
		if err != nil {
			return false, errPeers, err
		}

		// wait for block process finish
		errPeers, err = s.waitSyncFinished(bestHashElem, bestPeer)
		if err != nil {
			return false, errPeers, err
		}
	}

	s.log.Info("sync fix point finished")
	return true, nil, nil
}

func (s *Synchronizer) syncTreeBlockAndStates(peers []peer, bestPeer peer, hashFrom common.Hash, hashTo common.Hash, accHash common.Hash, hashTreeTimeout int, setHead bool, checkCheckPoint bool) ([]peer, error) {

	// sync hash tree and fix point
	hashTree, treePoint, err := s.syncHashTree(bestPeer, hashFrom, hashTo, accHash, hashTreeTimeout, checkCheckPoint)
	if err != nil {
		return []peer{bestPeer}, err
	}

	var peerMap = make(map[string]downloader.FetcherPeer)
	for _, peer := range peers {
		peerMap[peer.GetID()] = peer
	}

	//sync fix point blocks
	errPeers, err := s.syncFixPointBlocksForState(peerMap, treePoint, setHead)
	if err != nil {
		return errPeers, err
	}

	//sync state for fix point
	errPeers, err = s.syncPreStates(treePoint, peerMap)
	if err != nil {
		return errPeers, err
	}

	//sync common post blocks
	errPeers, err = s.syncCommonPostBlocks(peerMap, hashTree, treePoint.Height, setHead)
	if err != nil {
		return errPeers, err
	}
	return nil, nil
}

func (s *Synchronizer) checkCheckPoint(treePoint *types.TreePoint, hash common.Hash) bool {
	checkPoint := s.chain.GetCheckPointByHash(hash)
	if checkPoint != nil {
		if checkPoint.TreePoint.FullHash != treePoint.FullHash || checkPoint.TreePoint.Height != treePoint.Height {
			s.log.Error("check checkpoint failed, tree point is not equal", "treePoint", treePoint, "localTreePoint", checkPoint.TreePoint)
			return false
		}
	}
	return true
}

func (s *Synchronizer) syncHashTree(bestPeer peer, hashFrom common.Hash, hashTo common.Hash, accHash common.Hash, hashTreeTimeout int, checkCheckPoint bool) (*types.HashTree, *types.TreePoint, error) {
	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointHashTree)

	err := bestPeer.RequestSyncHashTree(hashFrom, hashTo)
	if err != nil {
		s.log.Error("request hashTree failed", "err", err)
		return nil, nil, errPeer
	}

	// process hash tree
	timeout := time.NewTimer(time.Duration(hashTreeTimeout) * time.Microsecond)
	select {
	case <-timeout.C:
		s.log.Error("request hashTree timeout")
		return nil, nil, errPeer
	case hashTreeRsp := <-s.hashTreeRevCh:
		hashTree := &hashTreeRsp.HashTree
		treePoint := &hashTreeRsp.TreePoint
		if treePoint.FullHash == (common.Hash{}) {
			s.log.Error("sync hash tree failed, struct is null", "hashTreeRsp", hashTreeRsp)
			return nil, nil, errPeer
		}

		//check checkpoint
		if checkCheckPoint && !s.checkCheckPoint(treePoint, hashFrom) {
			return nil, nil, errPeer
		}

		calcAccHash, err := hashTree.CalcAccHash(treePoint.RetrieveAccHashMap())
		if err != nil {
			s.log.Error("sync hash tree calc acc hash failed", "err", err)
			return nil, nil, errPeer
		}

		if calcAccHash != accHash {
			s.log.Error("sync hash tree calc acc hash failed, acc hash not equal", "calcAccHash", calcAccHash, "accHash", accHash)
			return nil, nil, errPeer
		}

		s.log.Info("verify hashTree succeed", "treePoint", hashTreeRsp.TreePoint)
		return hashTree, treePoint, nil
	}
}

func (s *Synchronizer) syncFixPointBlocksForState(peerMap map[string]downloader.FetcherPeer, treePoint *types.TreePoint, setHead bool) ([]peer, error) {
	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointPreBlocks)

	hashes := treePoint.UnRepeatedHashes()

	var peersErr []peer
	var blockCh = make(chan *types.Block)
	s.blockSync = downloader.StartFetchBlocksByHash(hashes, peerMap, func(id string, addBlack bool) {
		peersErr = append(peersErr, peerMap[id].(peer))
		s.removePeerCallback(id, addBlack)
	}, true, protocol.SyncStageFastSync, s.chain, blockCh)

	quitCh := make(chan struct{})
	receivedHashLength := 0
	var fixPointTo *types.Block

	go func() {
	ForLoop:
		for {
			select {
			case block := <-blockCh:
				s.log.Info("tree point info", "blockHash", block.FullHash(), "blockHeight", block.Header.Height)
				accHash := treePoint.RetrieveAccHashMap()[block.FullHash()]
				block.AccHash = accHash
				s.chain.InsertBlockNoCheck(block)

				if block.FullHash() == treePoint.MainChainHashList[len(treePoint.MainChainHashList)-1] {
					fixPointTo = block
				}
				receivedHashLength++
				if receivedHashLength == len(hashes) {
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

	if err != nil {
		s.log.Error("syncBlock for fixPoint failed", "hashFrom", treePoint.FullHash)
		return peersErr, errPeer
	}

	if setHead {
		s.lastHeadBlock = s.chain.CurrentBlock()
		s.chain.SetCurrentBlock(fixPointTo)
	}

	return nil, nil
}

func (s *Synchronizer) syncPreStates(treePoint *types.TreePoint, peerMap map[string]downloader.FetcherPeer) ([]peer, error) {
	s.log.Info("request for state sync", "len(mainList)", len(treePoint.MainChainHashList), "peers", peerMap)
	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointPreStates)

	var blocks types.Blocks
	for _, hash := range treePoint.MainChainHashList {
		block := s.chain.GetBlock(hash)
		blocks = append(blocks, block)
	}

	// state sync
	for _, block := range blocks {
		// todo: blocks should be sorted first
		root := block.Header.StateHash
		s.log.Info("start sync state", "rootHash", root, "Height", block.Header.Height, "fullHash", block.FullHash())

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
			return peersErr, errPeer
		}

		// set state flag as if it is not executed
		dbaccessor.WriteBlockStateCheck(s.chain.Database(), block.FullHash(), types.HasBlockStateButNotChecked)

		select {
		case <-s.fastSyncQuitCh:
			s.log.Info("RequestBlocksForPostStateSync from peer fastSyncQuitCh force quit")
			return nil, errors.New("sync recv quit")
		default:
		}
	}

	return nil, nil
}

func (s *Synchronizer) syncCommonPostBlocks(peerMap map[string]downloader.FetcherPeer, tree *types.HashTree, lowestHeight uint64, setHead bool) ([]peer, error) {
	// request for post block
	s.log.Info("sync post blocks", "treeLength", len(tree.Elems))

	addedSet := mapset.NewSet()

	hashes := tree.PostOrderTraversal(tree.RootIndex, addedSet)
	//hashesString, _ := json.Marshal(hashes)
	s.log.Info("post order hash tree", "len(hashes)", len(hashes))
	s.log.Info("post block hash tree", "len(tree)", len(tree.Elems), "rootIndex", tree.RootIndex)

	//filter hashes
	hashes = s.chain.Filter(hashes)
	s.log.Info("post order hash tree after filter", "len(hashes)", len(hashes))

	mainChainSet, _ := tree.RetrieveMainChainSet()

	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointPostBlocks)
	var peersErr []peer
	var blockCh = make(chan *types.Block)
	s.blockSync = downloader.StartFetchBlocksByHash(hashes, peerMap, func(id string, addBlack bool) {
		peersErr = append(peersErr, peerMap[id].(peer))
		s.removePeerCallback(id, addBlack)
	}, true, protocol.SyncStageFastSync, s.chain, blockCh)

	quitCh := make(chan struct{})
	go func() {
		cursor := NewCursor(hashes, mainChainSet, s.chain, s.packer, lowestHeight, setHead)
	ForLoop:
		for {
			select {
			case block := <-blockCh:
				err := cursor.ProcessBlock(block)
				if err != nil {
					s.log.Info("execute block failed", "err", err, "blockHash", block.FullHash(), "blockHeight", block.Header.Height)
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
		s.log.Error("syncBlock for post failed")
		return peersErr, errPeer
	}
	s.log.Info("Blocks received and executed")

	return nil, nil
}

func (s *Synchronizer) syncBestBlocks(bestHashElem protocol.HashElem, commonHighestHashElem protocol.HashElem, bestPeer peer) ([]peer, error) {
	// set status for fast sync
	s.changeFastSyncStatus(FastSyncStatusFixPointBestBlocks)
	var peerMap = make(map[string]downloader.FetcherPeer)
	peerMap[bestPeer.GetID()] = bestPeer

	s.log.Info("request best peer blocks", "hashTo", commonHighestHashElem, "bestHashElem", bestHashElem)
	err := bestPeer.RequestSyncBestPeerBlocks(commonHighestHashElem, bestHashElem)
	if err != nil {
		s.log.Error("request higher post blocks failed", "peer", bestPeer.Name(), "bestHash", bestHashElem, "err", err)
		return []peer{bestPeer}, errPeer
	}
	for {
		var finished = false
		timeout := time.NewTimer(time.Duration(s.config.ShortTimeOutOfSyncVeryHigh) * time.Microsecond)
		select {
		case <-timeout.C:
			s.log.Error("request blocks from best peer timeout", "peer", bestPeer.Name(), "hashTo", commonHighestHashElem, "bestHash", bestHashElem, "err", "timeout")
			return []peer{bestPeer}, errPeer
		case blocks := <-s.blocksForPostStateRevCh:
			// finish and exec
			if len(blocks) <= 0 {
				finished = true
			} else {
				s.log.Info("request blocks from best peer received", "len(Blocks)", len(blocks))
				for _, block := range blocks {
					s.log.Info("best peer block received", "hash", block.FullHash(), "height", block.Header.Height)
					s.blockProcessCh <- &network.BlockWithVerifyFlag{Block: block, Verify: true}
				}
			}
		case <-s.fastSyncQuitCh:
			s.log.Info("fastSync force quit")
			return nil, errors.New("fastSync force quit")
		}
		if finished {
			break
		}
	}

	return nil, nil
}

func (s *Synchronizer) waitSyncFinished(bestHashElem protocol.HashElem, bestPeer peer) ([]peer, error) {
	s.log.Info("wait for highest block process finish", "bestHashElem", bestHashElem, "bestPeer", bestPeer)
	timeout := time.NewTimer(time.Duration(s.config.LongTimeOutOfFixPointFinish) * time.Microsecond)
	for {
		if s.chain.HasBlock(bestHashElem.Hash) {
			s.log.Info("bestHash processed")
			break
		}

		//consume quitCh
		select {
		case <-timeout.C:
			s.log.Error("wait for highest block timeout", "peer", bestPeer.Name(), "bestHash", bestHashElem)
			return []peer{bestPeer}, errPeer

		case <-s.fastSyncQuitCh:
			s.log.Info("wait for highest block force quit")
			return nil, errors.New("sync recv quit")
		default:
		}
		time.Sleep(1 * time.Second)
	}
	return nil, nil
}
