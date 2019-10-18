package sync

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/ftl/protocol"
)

func getBestPeerByHead(peers []peer) peer {
	var bestPeer peer
	for _, p := range peers {
		_, simpleHash, height, round := p.Head()
		if bestPeer == nil || bestPeer.CompareTo(simpleHash, height, round) < 0 {
			bestPeer = p
		}
	}
	return bestPeer
}

func getBestPeerByHashes(peers []peer, peerHashes map[string]protocol.HashElems) (peer, protocol.HashElem) {
	var bestPeer peer
	var hashElemRes = protocol.HashElem{}
	for _, p := range peers {
		if hashElems, ok := peerHashes[p.GetID()]; ok {
			for _, hashElem := range hashElems {
				if hashElemRes == (protocol.HashElem{}) {
					hashElemRes = *hashElem
					bestPeer = p
					continue
				}
				if hashElemRes.CompareTo(*hashElem) < 1 {
					hashElemRes = *hashElem
					bestPeer = p
				}
			}
		}
	}
	return bestPeer, hashElemRes
}

//return common prefix from lowestHash to highestHash
// TODO: add test for this function
func getCommPreFromShortList(hashLists []protocol.HashElems) (protocol.HashElem, protocol.HashElem, error) {
	//	//if len(shortHashList)=30, make sure we have common prefix in  the lower [0,10]
	lowerHashMap := make(map[protocol.HashElem]int)
	for _, hashList := range hashLists {
		//we have checked len(hashList) ==s.config.shortHashListLength
		for i := len(hashList) / 2; i < len(hashList); i++ {
			if _, ok := lowerHashMap[*hashList[i]]; ok {
				lowerHashMap[*hashList[i]] = lowerHashMap[*hashList[i]] + 1
			} else {
				lowerHashMap[*hashList[i]] = 1
			}
		}
	}

	var haveLowerCmPrefix = false
	for _, v := range lowerHashMap {
		if v == len(hashLists) {
			haveLowerCmPrefix = true
			break
		}
	}
	if !haveLowerCmPrefix {
		return protocol.HashElem{}, protocol.HashElem{}, errNoCommonPrefixInShortHashLists
	}

	var lowestCommonHash = protocol.HashElem{Height: ^uint64(0)}
	var highestCommonHash = protocol.HashElem{Height: uint64(0)}
	var hash2HashElemMap = make(map[common.Hash]protocol.HashElem)
	hashMap := make(map[common.Hash]int)
	for _, hashList := range hashLists {
		for _, hashE := range hashList {
			if _, ok := hashMap[hashE.Hash]; ok {
				hashMap[hashE.Hash] = hashMap[hashE.Hash] + 1
			} else {
				hashMap[hashE.Hash] = 1
			}
			hash2HashElemMap[hashE.Hash] = *hashE
		}
	}
	for k, v := range hashMap {
		if v == len(hashLists) {
			if hash2HashElemMap[k].Height < lowestCommonHash.Height {
				lowestCommonHash = hash2HashElemMap[k]
			}
			if hash2HashElemMap[k].Height > highestCommonHash.Height {
				highestCommonHash = hash2HashElemMap[k]
			}
		}
	}
	return lowestCommonHash, highestCommonHash, nil
}
