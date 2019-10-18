// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync spiral contains the implementation of fractal sync complex mode.
package sync

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
)

func (s *Synchronizer) syncSkeletonLists(peers []peer, from protocol.HashElem) (map[string]protocol.HashElems, error) {
	log.Info("start to sync skeleton list", "peers", peers, "from", from)

	s.changeFastSyncStatus(FastSyncStatusLongHashList)

	fetcher := newLongHashFetcher(peers, protocol.SyncStageFastSync, 0, from, protocol.HashElem{},
		s.syncHashListChForFastSync, s.config.LongTimeOutOfLongList, s.removePeerCallback, s.log)
	res := fetcher.fetch()
	return fetcher.fetchResult.hashes, res
}

// hashList should be the same at checkpoint ,and may diff after
// long list is reverse order , smooth it first then compare it,result map is diff index ,string is bad PeerID
// bool means whether Interval Hash list should include the head block
func (s *Synchronizer) getCommPreFromLongList(hashLists map[string]protocol.HashElems, interval uint64) (map[string]int, error) {
	var currentBlock *protocol.HashElem
	lengthOfHL := ^uint64(0)
	var shortestHashList protocol.HashElems
	var peerIndexMap = make(map[string]int)

	for _, hashList := range hashLists {
		if uint64(len(hashList)) < lengthOfHL {
			shortestHashList = hashList
			lengthOfHL = uint64(len(hashList))
		}
	}

	for index := len(shortestHashList) - 1; index >= 0; index-- {
		currentBlock = shortestHashList[index]
		commonCount := 0

		for peerId, hashList := range hashLists {
			index1 := len(hashList) - (len(shortestHashList) - index)
			peerIndexMap[peerId] = index1
			if hashList[index1].String() == currentBlock.String() {
				commonCount++
			}
		}
		if commonCount != len(hashLists) {
			return peerIndexMap, nil
		}
	}
	// if no diff
	s.log.Info("no diff in longHashLists", "peerIndexMap", peerIndexMap, "headOrNot", true)
	return peerIndexMap, nil
}

//func (s *Synchronizer) getCommPreFromIntervalList(hashListsMap map[string]protocol.HashElems) (map[string]int, error) {
//	s.log.Info("get common prefix from internal hash list")
//	var fromBlock *protocol.HashElem
//	lengthOfHL := ^uint64(0)
//	var shortestHashList []*protocol.HashElem
//
//	//get shortest HashList
//	for _, hashList := range hashListsMap {
//		if uint64(len(hashList)) < lengthOfHL {
//			shortestHashList = hashList
//			lengthOfHL = uint64(len(hashList))
//		}
//	}
//	//get peerIndexMap which diff
//	for index := len(shortestHashList) - 1; index >= 0; index-- {
//		fromBlock = shortestHashList[index]
//		commonCount := 0
//		var peerIndexMap = make(map[string]int)
//		for peerId, hashList := range hashListsMap {
//			index1 := len(hashList) - (len(shortestHashList) - index)
//			peerIndexMap[peerId] = index1
//			if hashList[index1].String() == fromBlock.String() {
//				commonCount++
//			}
//		}
//
//		if commonCount != len(hashListsMap) {
//			s.log.Info("get common prefix from internal hash list succeed", "peerIndexMapDiff", peerIndexMap)
//			return peerIndexMap, nil
//		}
//	}
//	s.log.Error("Interval Hash list can't be the same")
//	return nil, errors.New("Interval Hash list can't be the same")
//
//}

// if count(peer of common prefix) > =commonPrefixCount and is the best peer then return peers
func (s *Synchronizer) findMainChainPeers(hashListMap map[string]protocol.HashElems, peers []peer, peerIndexMap map[string]int, comPrefixCount int) ([]peer, map[string]protocol.HashElems, protocol.HashElem, protocol.HashElem, protocol.HashElems, error) {
	s.log.Info("find main chain peers", "peerIndexMap", peerIndexMap, "comPrefixCount", comPrefixCount)
	var countStatistic = make(map[string]int)
	var hash2Peers = make(map[string][]string) //hash to peer array

	var bestHashElem = &protocol.HashElem{Height: 0, Hash: common.Hash{}, Round: 0}
	var bestHashElemBelow = &protocol.HashElem{}
	var resultPeers []peer
	var resultHashListMap = make(map[string]protocol.HashElems)

	// compare who is the best hashElem
	for peerId, v := range hashListMap {
		index := peerIndexMap[peerId] - s.config.CheckMainChainPostBlockLength + 1
		if index <= 0 {
			s.log.Error("peer is bad, (index-20+1) <=0", "peer", peerId, "index", index)
			continue
		}
		hashElem := v[index]
		if _, ok := countStatistic[hashElem.String()]; ok {
			countStatistic[hashElem.String()] = countStatistic[hashElem.String()] + 1
		} else {
			countStatistic[hashElem.String()] = 1
		}
		if _, ok := hash2Peers[hashElem.String()]; ok {
			hash2Peers[hashElem.String()] = append(hash2Peers[hashElem.String()], peerId)
		} else {
			hash2Peers[hashElem.String()] = []string{peerId}
		}
		if bestHashElem.CompareTo(*hashElem) < 0 {
			bestHashElem = hashElem
		}
	}
	// get a consensus of hashElem list
	var bestHashElemStr string
	for k, v := range countStatistic {
		if v >= comPrefixCount && k == bestHashElem.String() {
			bestHashElemStr = k
			s.log.Info("find main chain peers", "hash", k, "besthash", bestHashElem)
			break
		}
	}

	if bestHashElemStr == "" {
		s.log.Error("can't get common prefix", "countStatistic", countStatistic)
		return nil, nil, protocol.HashElem{}, protocol.HashElem{}, nil, errCanNotGetConsensus
	}

	// collect peers
	for _, peerId := range hash2Peers[bestHashElemStr] {
		for _, peer := range peers {
			if peerId == peer.GetID() {
				resultPeers = append(resultPeers, peer)
				break
			}
		}
	}
	// get from to
	var blockSyncHashList protocol.HashElems
	for _, peer := range resultPeers {
		resultHashListMap[peer.GetID()] = hashListMap[peer.GetID()]
		if len(blockSyncHashList) == 0 {
			//index >0 ,it has been  checked
			indexFrom := peerIndexMap[peer.GetID()]
			indexTo := peerIndexMap[peer.GetID()] - s.config.CheckMainChainPostBlockLength + 1
			bestHashElemBelow = resultHashListMap[peer.GetID()][indexFrom]
			for i := indexFrom; i >= indexTo; i-- {
				blockSyncHashList = append(blockSyncHashList, resultHashListMap[peer.GetID()][i])
			}
		}
	}

	s.log.Info("find main chain peers succeed", "mainChainPeers", resultPeers, "from", *bestHashElemBelow, "to", *bestHashElem, "blockSyncHashList", blockSyncHashList)
	return resultPeers, resultHashListMap, *bestHashElemBelow, *bestHashElem, blockSyncHashList, nil
}
