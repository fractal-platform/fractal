package sync

import (
	"errors"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/protocol"
)

func (s *Synchronizer) doFastSync() {
	s.log.Info("start fast sync")

	s.chain.StopCreateCheckPoint()

	if s.miner != nil {
		s.miner.Stop()
	}

	//stop cp2fp
	s.cp2fp.stopAll()

	peers := s.getPeers()
	// close all fast sync goroutines
	close(s.fastSyncQuitCh)
	s.fastSyncQuitCh = make(chan struct{})

	//sync short hashLists
	honestPeers, peerShortHashListMap, err := s.syncShortHashListsForFastSync(peers, s.config.MinFastSyncPeerCount)
	if err != nil {
		s.fastSyncErrCh <- struct{}{}
		return
	}

	treePoint := s.getLatestCheckPoint()

	var complexSync = false
	var hasSkeleton = false
	var peerLongHashListMap = make(map[string]protocol.HashElems)
	// if diff is not in the top Blocks , do the loop
	for len(honestPeers) > 0 {
		//do syncFixPoint
		var fixPoint protocol.HashElem
		honestPeers, fixPoint, highestPoint, err := s.doSimpleFastSync(honestPeers, peerShortHashListMap)
		if err == nil {
			s.log.Info("doFastSync syncFixPoint succeed")
			s.fixPoint = fixPoint

			//sync from break point or checkpoint
			go s.cp2fp.startTask(highestPoint.Height, highestPoint.Hash, highestPoint.AccHash, honestPeers)

			s.fastSyncFinishedCh <- struct{}{}
			return
		} else if err.Error() == errNoCommonPrefixInShortHashLists.Error() {
			s.log.Info("fast sync get into complex sync")
			complexSync = true
			s.changeFastSyncMode(FastSyncModeComplex)

			//find fix point using long hash list first ,then interval hash list
			interHashesMap, peerIndexIntervalMap, err := s.findFixpoint(hasSkeleton, honestPeers, treePoint, peerLongHashListMap)
			if err != nil {
				s.log.Error("find fix point failed", "err", err)
				s.fastSyncErrCh <- struct{}{}
				return
			}
			hasSkeleton = true

			// get main chain peers
			s.changeFastSyncStatus(FastSyncStatusCheckMainChain)
			leftPeers, err := s.findAndCheckMainChain(interHashesMap, honestPeers, s.config.CommonPrefixCount, peerIndexIntervalMap)
			if err != nil {
				s.log.Error("find and check main chain failed", "err", err)
				s.fastSyncErrCh <- struct{}{}
				return
			}
			// refresh honestPeers and honestPeerMap
			honestPeers = leftPeers
			s.log.Info("finish complex round", "len(leftPeers)", len(leftPeers), "len(honestPeers)", len(honestPeers))
		} else if err.Error() == errPeer.Error() {
			if !complexSync {
				s.fastSyncErrCh <- struct{}{}
				s.log.Error("not enough peer,need to stop", "err", err)
				return
			} else {
				s.log.Error("complexSync have identified who are honest,left honest and connect peer should go on", "err", err)
				continue
			}
		} else {
			//force quit
			s.fastSyncErrCh <- struct{}{}
			s.log.Error("need to force quit", "err", err)
			return
		}
	}
}

//
func (s *Synchronizer) findFixpoint(hasSkeleton bool, peers []peer, checkpoint types.TreePoint, peerSkeletonMap map[string]protocol.HashElems) (map[string]protocol.HashElems, map[string]int, error) {
	s.log.Info("start to find fix point", "hasSkeleton", hasSkeleton, "peers", peers, "checkpoint", checkpoint, "len(peerSkeletonMap)", len(peerSkeletonMap))
	var peerLongListIndexMap map[string]int
	var honestPeerLongHashLists map[string]protocol.HashElems
	var err error

	peerLongListIndexMap, honestPeerLongHashLists, err = s.findCommonFromSkeletonLists(hasSkeleton, peers,
		protocol.HashElem{Height: checkpoint.Height, Hash: checkpoint.FullHash, Round: 0, AccHash: common.Hash{}}, peerSkeletonMap)
	if err != nil {
		s.log.Error("get common skeleton lists failed")
		return nil, nil, err
	}

	for peerId, index := range peerLongListIndexMap {
		s.log.Info("find common prefix for long hash lists", "peerId", peerId, "index", index, "hashElem", honestPeerLongHashLists[peerId][index])
	}

	return honestPeerLongHashLists, peerLongListIndexMap, nil
}

//sync short hashLists, if there are not enough good peers, return false
func (s *Synchronizer) syncShortHashListsForFastSync(peers []peer, peerThreshold int) ([]peer, map[string]protocol.HashElems, error) {
	s.log.Info("start to sync short hash list")

	// set mode & status for fast sync
	s.changeFastSyncMode(FastSyncModeEasy)
	s.changeFastSyncStatus(FastSyncStatusShortHashList)

	fetcher := newShortHashFetcher(peers, protocol.SyncStageFastSync, s.config.ShortHashListLength, true, peerThreshold, true,
		s.syncHashListChForFastSync, s.config.ShortTimeOutOfShortLists, s.removePeerCallback, s.log)
	res := fetcher.fetch()
	return fetcher.fetchResult.honestPeers, fetcher.fetchResult.hashes, res
}

func (s *Synchronizer) findCommonFromSkeletonLists(hasSkeleton bool, peers []peer, from protocol.HashElem, peerLongHashListMap map[string]protocol.HashElems) (map[string]int, map[string]protocol.HashElems, error) {
	//
	if !hasSkeleton {
		//sync long HashList with Interval
		peerSkeletonMap, err := s.syncSkeletonLists(peers, from)
		if err != nil {
			s.log.Error("find fix point failed", "err", err)
			return nil, nil, err
		}
		for _, peer := range peers {
			peerLongHashListMap[peer.GetID()] = peerSkeletonMap[peer.GetID()]
		}
	}

	//get common prefix of honest peers long hashList
	var honestPeerLongHashLists = make(map[string]protocol.HashElems)
	for _, peer := range peers {
		honestPeerLongHashLists[peer.GetID()] = peerLongHashListMap[peer.GetID()]
	}

	//get common prefix of honest peers long hashList
	peerLongListIndexMap, err := s.getCommPreFromLongList(honestPeerLongHashLists, uint64(s.config.Interval))
	if err != nil {
		s.log.Error("find fix point failed", "err", err)
		return nil, nil, err
	}
	return peerLongListIndexMap, honestPeerLongHashLists, nil
}

//find fix point, and then sync fix point
//return []peer(left honest peers)
//return err: errNoCommonPrefixInShortHashLists , errPeer ,forceQuit
func (s *Synchronizer) doSimpleFastSync(honestPeers []peer, peerShortHashListMap map[string]protocol.HashElems) ([]peer, protocol.HashElem, protocol.HashElem, error) {
	s.log.Info("do simple fast sync", "honestPeerCount", len(honestPeers), "peerShortHashListMapSize", len(peerShortHashListMap))

	//set honestPeers left
	var honestShortHashLists []protocol.HashElems
	var leftHonestPeers []peer
	for _, peer := range honestPeers {
		leftHonestPeers = append(leftHonestPeers, peer)
		honestShortHashLists = append(honestShortHashLists, peerShortHashListMap[peer.GetID()])
	}

	//get common prefix
	lowestCommonHash, highestCommonHash, err := getCommPreFromShortList(honestShortHashLists)
	s.log.Info("get common hash from short hash list", "lowestCommonHash", lowestCommonHash, "highestCommonHash", highestCommonHash, "error", err)
	if err != nil {
		return leftHonestPeers, lowestCommonHash, highestCommonHash, err
	}

	//get positive seq of common hashList
	blockSyncHashList := getHashListPositiveSeqByHashFromHashTo(honestShortHashLists[0], lowestCommonHash, highestCommonHash)

	if len(blockSyncHashList) <= types.HashTreeMinLength {
		s.log.Error("common short hash list length wrong", "len(common)", len(blockSyncHashList))
		return leftHonestPeers, lowestCommonHash, highestCommonHash, err
	}

	//sync fixPoint
	bestPeer, bestHashElem := getBestPeerByHashes(leftHonestPeers, peerShortHashListMap)
	check, badPeers, err := s.syncFixPointAndBest(honestPeers, bestPeer, blockSyncHashList, bestHashElem, true)
	if err != nil || !check {
		s.log.Error("sync fix point failed", "err", err, "check", check, "badPeers", badPeers)
		if err.Error() == errPeer.Error() {
			for _, peer := range badPeers {
				s.removePeerCallback(peer.GetID(), false)
				for i, hPeer := range leftHonestPeers {
					if hPeer.GetID() == peer.GetID() {
						leftHonestPeers = append(leftHonestPeers[:i], leftHonestPeers[i+1:]...)
						break
					}
				}
			}
			return leftHonestPeers, lowestCommonHash, highestCommonHash, err
		} else {
			return leftHonestPeers, lowestCommonHash, highestCommonHash, errors.New("need to stop fast sync")
		}
	}
	return leftHonestPeers, lowestCommonHash, highestCommonHash, nil
}

//return a hashList of if there is already common prefix of hashLists
func getHashListPositiveSeqByHashFromHashTo(hashList protocol.HashElems, hashEFrom protocol.HashElem, hashETo protocol.HashElem) protocol.HashElems {
	var hashFromIndex int
	var hashToIndex int
	var result protocol.HashElems
	for index, hashE := range hashList {
		if hashE.Hash == hashEFrom.Hash {
			hashFromIndex = index
		}
		if hashE.Hash == hashETo.Hash {
			hashToIndex = index
		}
	}
	for i := hashFromIndex; i >= hashToIndex; i-- {
		result = append(result, hashList[i])
	}
	return result
}
