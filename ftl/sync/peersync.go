// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package sync contains the implementation of fractal sync strategy.
package sync

import (
	"errors"
	"time"

	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
)

func (s *Synchronizer) DoPeerSync(p *network.Peer) {
	s.log.Info("start to do peer sync", "peer", p)
	status := s.GetSyncStatus()
	if status == SyncStatusPeerSync || status == SyncStatusFastSync {
		s.log.Info("peer sync over(sync has already started, do not need to do peer sync)")
		return
	}

	s.chain.StopCreateCheckPoint()

	// start peer sync
	s.changeSyncStatus(SyncStatusPeerSync)
	s.peerSyncStarted[p.GetID()] = true

	shortHashLists, peerShortHashListMap, err := s.getPeerSyncShortHashes(p)
	if err != nil {
		s.log.Error("peer sync failed(get peer short hashes failed)", "peer", p.GetID(), "error", err.Error())
		s.peerSyncErrCh <- p
		return
	}

	// find the last round
	peerHashElems := peerShortHashListMap[p.GetID()]
	roundTo := peerHashElems[0].Round

	// stop miner
	// TODO it's not the best choice. we must make sure the peer is honest player, or it may suspend our mining.
	if s.miner != nil {
		s.miner.Stop()
	}

	//stop cp2fp
	s.cp2fp.stopAll()

	//get common prefix
	lowestCommonHash, highestCommonHash, err := getCommPreFromShortList(shortHashLists)
	s.log.Info("get common hash from short hash list", "lowestCommonHash", lowestCommonHash, "highestCommonHash", highestCommonHash, "error", err)
	if err == nil {
		err = s.peerSyncSimple(p, roundTo, lowestCommonHash, highestCommonHash, peerShortHashListMap)
		if err != nil {
			s.log.Error("peer sync failed", "peer", p.GetID(), "error", err.Error())
			s.peerSyncErrCh <- p
		} else {
			s.log.Info("peer sync over")
			s.peerSyncFinishedCh <- p
		}
		return
	}

	// reset flag and do callback & change to fast sync
	s.peerSyncStarted[p.GetID()] = false
	if status == SyncStatusPeerSync || status == SyncStatusFastSync {
		s.log.Info("peer sync over(sync has already started, do not need to do fast sync)")
		return
	}
	s.changeSyncStatus(SyncStatusFastSync)
	s.doFastSync()
}

func (s *Synchronizer) getPeerSyncShortHashes(p *network.Peer) ([]protocol.HashElems, map[string]protocol.HashElems, error) {
	//sync short hashLists
	_, peerShortHashListMap, err := s.syncShortHashListsForPeerSync([]peer{p}, 1)
	if err != nil {
		s.log.Error("peer sync short hash list failed", "err", err)
		return nil, nil, err
	}

	currentShortHashes, err := s.getLocalShortHashes()
	if err != nil {
		s.log.Error("peer sync local short hash list failed", "err", err)
		return nil, nil, err
	}

	peerShortHashListMap[selfPeerId] = currentShortHashes

	var shortHashLists []protocol.HashElems
	shortHashLists = append(shortHashLists, currentShortHashes)
	shortHashLists = append(shortHashLists, peerShortHashListMap[p.GetID()])
	return shortHashLists, peerShortHashListMap, nil
}

//sync short hashLists, if there are not enough good peers, return false
func (s *Synchronizer) syncShortHashListsForPeerSync(peers []peer, peerThreshold int) ([]peer, map[string]protocol.HashElems, error) {
	s.log.Info("start to sync short hash list", "peers", peers, "threshold", peerThreshold)

	fetcher := newShortHashFetcher(peers, protocol.SyncStagePeerSync, s.config.ShortHashListLength, true, peerThreshold, true,
		s.syncHashListChForPeerSync, s.config.ShortTimeOutOfShortLists, s.removePeerCallback, s.log)
	res := fetcher.fetch()
	return fetcher.fetchResult.honestPeers, fetcher.fetchResult.hashes, res
}

func (s *Synchronizer) peerSyncSimple(p *network.Peer, roundTo uint64, lowestCmHashElem protocol.HashElem, highestCommonHash protocol.HashElem, peerShortHashListMap map[string]protocol.HashElems) error {
	s.log.Info("start to do peer simple sync", "peer", p)

	//
	var peerMap = make(map[string]downloader.FetcherPeer)
	peerMap[p.GetID()] = p
	//
	highestCommonBlock := s.chain.GetBlock(highestCommonHash.Hash)
	if highestCommonBlock != nil {
		s.chain.SetCurrentBlock(highestCommonBlock)
	}

	//do fullFill
	var blockCh = make(chan *types.Block)
	var peersErr []peer

	s.blockSyncRound = downloader.StartFetchBlocksByRound(lowestCmHashElem.Round-1, roundTo, peerMap, func(id string, addBlack bool) {
		peersErr = append(peersErr, peerMap[id].(peer))
		s.removePeerCallback(id, addBlack)
	}, true, protocol.SyncStagePeerSync, s.chain, blockCh)

	var blockSyncHashList protocol.HashElems
	for i := len(peerShortHashListMap[p.GetID()]) - 1; i >= 0; i-- {
		blockSyncHashList = append(blockSyncHashList, peerShortHashListMap[p.GetID()][i])
	}

	quitCh := make(chan struct{})
	timeout := time.NewTimer(time.Second * 600)
	isTimeout := false
	go func() {
		cursor := NewCursorRound(blockSyncHashList, s.chain, s.packer, true, s.lengthForStatesSync())
	ForLoop:
		for {
			select {
			case block := <-blockCh:
				timeout.Reset(time.Second * 600)
				err := cursor.ProcessBlock(block)
				if err == errMainBlockCheckAndExecFailed {
					break ForLoop
				}
				if cursor.IsFinished() {
					break ForLoop
				}
			case <-timeout.C:
				isTimeout = true
				break ForLoop
			case <-quitCh:
				return
			}
		}
		s.blockSyncRound.Finish()
	}()

	err := s.blockSyncRound.Wait()
	s.blockSyncRound = nil
	close(quitCh)
	if err != nil {
		s.log.Error("peer sync failed", "hashFrom", lowestCmHashElem, "hashToRound", roundTo, "hashList", blockSyncHashList)
		return err
	}
	if isTimeout {
		s.log.Error("peer sync timeout", "hashFrom", lowestCmHashElem, "hashToRound", roundTo, "hashList", blockSyncHashList)
		return errors.New("peer sync timeout")
	}
	s.log.Info("peer sync finish")
	return nil
}
