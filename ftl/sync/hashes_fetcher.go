// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// sync check contains the implementation of fractal fetch hashes.
package sync

import (
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
	"time"
)

type fetchResult struct {
	honestPeers []peer
	hashes      map[string]protocol.HashElems
}

type hashFetcher struct {
	peers     []peer
	syncStage protocol.SyncStage
	syncType  protocol.SyncHashType

	// for hash list
	from         protocol.HashElem
	to           protocol.HashElem
	length       int
	threshold    bool
	thresholdNum int
	ch           chan PeerHashElemList

	// serial or parallel
	serial bool

	timeout  int
	removeFn removePeerCallback
	log      log.Logger

	fetchResult *fetchResult
}

func newShortHashFetcher(peers []peer, syncStage protocol.SyncStage, length int, threshold bool, thresholdNum int, serial bool,
	ch chan PeerHashElemList, timeout int, removePeerFn removePeerCallback, log log.Logger) *hashFetcher {
	return &hashFetcher{
		peers:     peers,
		syncStage: syncStage,
		syncType:  protocol.SyncHashTypeShort,

		length:       length,
		threshold:    threshold,
		thresholdNum: thresholdNum,
		serial:       serial,
		ch:           ch,
		timeout:      timeout,
		removeFn:     removePeerFn,
		log:          log,
		fetchResult:  &fetchResult{hashes: make(map[string]protocol.HashElems)},
	}
}

func newLongHashFetcher(peers []peer, syncStage protocol.SyncStage, length int, from protocol.HashElem, to protocol.HashElem,
	ch chan PeerHashElemList, timeout int, removePeerFn removePeerCallback, log log.Logger) *hashFetcher {
	return &hashFetcher{
		peers:     peers,
		syncStage: syncStage,
		syncType:  protocol.SyncHashTypeLong,

		length:      length,
		from:        from,
		to:          to,
		ch:          ch,
		timeout:     timeout,
		removeFn:    removePeerFn,
		log:         log,
		fetchResult: &fetchResult{hashes: make(map[string]protocol.HashElems)},
	}
}

func (h *hashFetcher) fetch() error {
	if h.threshold && len(h.peers) < h.thresholdNum {
		h.log.Error("threshold is enabled, not enough peers", "peerCount", len(h.peers), "threshold", h.thresholdNum)
		return errNotEnoughPeers
	}
	if h.serial {
		return h.serialFetch()
	} else {
		return h.parallelFetch()
	}
}

func (h *hashFetcher) serialFetch() error {
	for _, peer := range h.peers {
		err := peer.RequestSyncHashList(h.syncStage, h.syncType, h.from, h.to)
		if err != nil {
			h.log.Error("sync hash list failed", "peer", peer.GetID(), "stage", h.syncStage, "type", h.syncType, "err", err)
			h.removeFn(peer.GetID(), false)
			continue
		}

		timeout := time.NewTimer(time.Duration(h.timeout) * time.Microsecond)
		select {
		case <-timeout.C:
			h.log.Error("sync hash list timeout", "peer", peer.GetID(), "stage", h.syncStage, "type", h.syncType, )
			h.removeFn(peer.GetID(), false)
			continue
		case peerHashList := <-h.ch:
			h.log.Info("receive peer hashes", "peerHashList", peerHashList)
			if !h.checkHashes(peer, peerHashList) {
				h.log.Error("sync hash list check failed", "peer", peer.GetID(), "stage", h.syncStage, "type", h.syncType, )
				h.removeFn(peer.GetID(), false)
				continue
			}

			// add to result
			h.fetchResult.hashes[peer.GetID()] = peerHashList.HashList
			h.fetchResult.honestPeers = append(h.fetchResult.honestPeers, peer)
		}
		if h.threshold && len(h.fetchResult.honestPeers) >= h.thresholdNum {
			break
		}
	}

	if h.threshold && len(h.fetchResult.honestPeers) < h.thresholdNum {
		h.log.Error("serial fetch hashes, not enough peers", "stage", h.syncStage, "type", h.syncType, "len(peers)", len(h.fetchResult.honestPeers), "threshold", h.thresholdNum)
		return errNotEnoughPeers
	}

	h.log.Info("serial fetch hashes success", "stage", h.syncStage, "type", h.syncType, "peers", h.fetchResult.honestPeers, "hashes", len(h.fetchResult.hashes))
	return nil
}

func (h *hashFetcher) parallelFetch() error {
	for _, peer := range h.peers {
		err := peer.RequestSyncHashList(h.syncStage, h.syncType, h.from, h.to)
		// TODO: must modify log
		h.log.Info("parallel fetch", "h", *h)
		if err != nil {
			h.log.Error("parallel fetch hashes failed", "err", err, "peer", peer)
			return errNotEnoughPeers
		}
	}

	timeout := time.NewTimer(time.Duration(h.timeout) * time.Microsecond)
ForEnd:
	for range h.peers {
		select {
		case <-timeout.C:
			h.log.Error("parallel fetch hashes timeout")
			break ForEnd
		case peerHashList := <-h.ch:
			res := h.checkHashes(peerHashList.Peer, peerHashList)
			if !res {
				h.log.Error("sync hash list check failed", "peer", peerHashList.Peer.GetID())
				h.removeFn(peerHashList.Peer.GetID(), false)
				break ForEnd
			}

			// add to result
			h.fetchResult.hashes[peerHashList.Peer.GetID()] = peerHashList.HashList
			h.fetchResult.honestPeers = append(h.fetchResult.honestPeers, peerHashList.Peer)
		}
	}

	// judge again
	if len(h.fetchResult.honestPeers) < len(h.peers) {
		h.log.Error("not enough good peers", "len(honestPeers)", len(h.fetchResult.honestPeers), "len(peers)", len(h.peers))
		return errNotEnoughPeers
	}
	log.Info("parallel fetch hashes success", "peers", h.fetchResult.honestPeers, "hashes", len(h.fetchResult.hashes))
	return nil
}

func (h *hashFetcher) checkHashes(p peer, peerHashList PeerHashElemList) bool {
	switch peerHashList.HashType {
	case protocol.SyncHashTypeShort:
		return h.checkShortHashes(p, peerHashList.HashList)
	case protocol.SyncHashTypeLong:
		return h.checkLongHashes(p, peerHashList.HashList)
	default:
		log.Error("hash fetcher wrong hash type", "type", peerHashList.HashType)
		return false
	}
}

func (h *hashFetcher) checkShortHashes(p peer, hashes protocol.HashElems) bool {
	if len(hashes) <= 0 {
		h.log.Error("sync short hashes failed", "len(hashes)", len(hashes), "peer", p.GetID())
		return false
	}

	if len(hashes) != h.length {
		h.log.Error("short hash list length is not correct", "peer", p.GetID(), "shortHashListLength", len(hashes))
		return false
	}

	//check hashList
	for i := 0; i < len(hashes)-1; i++ {
		if hashes[i].Height != hashes[i+1].Height+1 {
			h.log.Error("short hash list check failed", "peer", p.GetID(), "hashList", hashes)
			return false
		}
	}
	return true
}

func (h *hashFetcher) checkLongHashes(p peer, hashes protocol.HashElems) bool {
	if len(hashes) <= 0 {
		h.log.Error("sync hashes failed", "stage", h.syncStage, "type", h.syncType, "len(hashes)", len(hashes), "peer", p.GetID())
		return false
	}

	if h.from != (protocol.HashElem{}) && hashes[len(hashes)-1].Hash != h.from.Hash {
		h.log.Error("hashes from is not right", "stage", h.syncStage, "type", h.syncType, "peerId", p.GetID(), "from", h.from, "hashes", len(hashes))
		return false
	}
	if h.to != (protocol.HashElem{}) && hashes[0].Hash != h.to.Hash {
		h.log.Error("hashes to is not right", "stage", h.syncStage, "type", h.syncType, "peerId", p.GetID(), "to", h.to, "hashes", len(hashes))
		return false
	}
	if h.length != 0 && len(hashes) != h.length {
		h.log.Error("hashes len  is not right", "stage", h.syncStage, "type", h.syncType, "peerId", p.GetID(), "hashes", len(hashes), "h.length", h.length)
		return false
	}
	for i := 0; i < len(hashes)-1; i++ {
		if hashes[i].Height != hashes[i+1].Height+1 {
			h.log.Error("hashes order or interval is wrong", "stage", h.syncStage, "type", h.syncType, "hashHigher", hashes[i], "hashLower", hashes[i+1])
			return false
		}
	}
	return true
}
