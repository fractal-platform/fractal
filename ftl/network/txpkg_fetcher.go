package network

import (
	"sync"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
)

type txpkgFetcher struct {
	peers       *Peers
	busyPeers   mapset.Set
	pendingReqs []common.Hash
	mutex       sync.Mutex
}

func newTxpkgFetcher(peers *Peers) *txpkgFetcher {
	f := &txpkgFetcher{
		peers:       peers,
		busyPeers:   mapset.NewSet(),
		pendingReqs: make([]common.Hash, 128),
	}
	return f
}

func (f *txpkgFetcher) insertTask(txpkgHash common.Hash) bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	peers := f.peers.PeersWithTxPackage(txpkgHash)
	for _, peer := range peers {
		if !f.busyPeers.Contains(peer.GetID()) {
			f.busyPeers.Add(peer.GetID())
			peer.RequestTxPackage(txpkgHash)
			return true
		}
	}

	f.pendingReqs = append(f.pendingReqs, txpkgHash)
	return false
}

func (f *txpkgFetcher) finishTask(peer *Peer) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	// release
	f.busyPeers.Remove(peer.GetID())

	// handle pending tasks
	for i, hash := range f.pendingReqs {
		if peer.HasTxPackage(hash) {
			f.busyPeers.Add(peer.GetID())
			f.pendingReqs = append(f.pendingReqs[:i], f.pendingReqs[i+1:]...)
			peer.RequestTxPackage(hash)
			return
		}
	}
}
