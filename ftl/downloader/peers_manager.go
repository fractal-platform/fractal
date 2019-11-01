package downloader

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/utils/log"
)

var (
	MinBlockRoundFetch = 500
	MaxBlockRoundFetch = 1000 // Amount of rounds of blocks to allow fetching per request
	MinBlockFetch      = 50
	MaxBlockFetch      = 100

	MinStateFetch = 1
	MaxStateFetch = 384 // Amount of node state values to allow fetching per request

	MinPkgsFetch     = 1
	MaxPkgsFetch     = 500              // Amount of kpgs to allow fetching per request
	RttMaxEstimate   = 60 * time.Second // Maximum Round-trip time to target for download requests
	QosConfidenceCap = 10               // Number of peers above which not to modify RTT confidence
	RttMinConfidence = 0.1              // Worse confidence factor in our estimated RTT value
	TtlScaling       = 3                // Constant scaling factor for RTT -> TTL conversion
	TtlLimit         = 90 * time.Second // Maximum TTL allowance to prevent reaching crazy timeouts
	BytesPerRound    = 3 * 100          // Average bytes of blocks per round
	BytesPerPkg      = 10 * 1024        // Average bytes of pkg
	BytesPerBlock    = 8 * 1024         // Average bytes of block
	qosTuningPeers   = 5                // Number of peers to tune based on (best peers)
	rttMinEstimate   = 2 * time.Second  // Minimum Round-trip time to target for download requests
)

var (
	errCancelStateFetch    = errors.New("state data download canceled (requested)")
	errBlockWithWrongRound = errors.New("blocks are not in the right Round range")
	errNoAvailPeer         = errors.New("no available peer")
	errAlreadyRegistered   = errors.New("peer is already registered")
	errNotRegistered       = errors.New("peer is not registered")
)

//peersManager manages the peers which used to fetch data for sync
type peersManager struct {
	peersLock sync.RWMutex
	peers     map[string]*Peer // Set of active peers from which fetch data

	rttEstimate   uint64 // Round trip time to target for download requests
	rttConfidence uint64 // Confidence in the estimated RTT (unit: millionths to allow atomic ops)

	// Callbacks
	dropPeer peerDropFn // Drops a peer for misbehaving

	//Recv peersmanager register/unregister peer msg
	newPeerCh chan struct{}
}

func newPeersManager(dropPeerFn peerDropFn) *peersManager {
	return &peersManager{
		dropPeer:      dropPeerFn,
		rttEstimate:   uint64(RttMaxEstimate),
		rttConfidence: uint64(1000000),
		newPeerCh:     make(chan struct{}),
		peers:         make(map[string]*Peer),
	}

}

// requestRTT returns the current target Round trip time for a download request
// to complete in.
//
// Note, the returned RTT is .9 of the actually estimated RTT. The reason is that
// the downloader tries to adapt queries to the RTT, so multiple RTT values can
// be adapted to, but smaller ones are preferred (stabler download stream).
func (pm *peersManager) requestRTT() time.Duration {
	return time.Duration(atomic.LoadUint64(&pm.rttEstimate)) * 9 / 10
}

// requestTTL returns the current timeout allowance for a single download request
// to finish under.
func (pm *peersManager) requestTTL() time.Duration {
	var (
		rtt  = time.Duration(atomic.LoadUint64(&pm.rttEstimate))
		conf = float64(atomic.LoadUint64(&pm.rttConfidence)) / 1000000.0
	)
	ttl := time.Duration(TtlScaling) * time.Duration(float64(rtt)/conf)
	if ttl > TtlLimit {
		ttl = TtlLimit
	}
	return ttl
}

// qosReduceConfidence is meant to be called when a new peer joins the downloader's
// peer set, needing to reduce the confidence we have in out QoS estimates.
func (pm *peersManager) qosReduceConfidence() {
	// If we have a single peer, confidence is always 1
	peers := uint64(len(pm.peers))
	if peers == 0 {
		// Ensure peer connectivity races don't catch us off guard
		return
	}
	if peers == 1 {
		atomic.StoreUint64(&pm.rttConfidence, 1000000)
		return
	}
	// If we have a ton of peers, don't drop confidence)
	if peers >= uint64(QosConfidenceCap) {
		return
	}
	// Otherwise drop the confidence factor
	conf := atomic.LoadUint64(&pm.rttConfidence) * (peers - 1) / peers
	if float64(conf)/1000000 < RttMinConfidence {
		conf = uint64(RttMinConfidence * 1000000)
	}
	atomic.StoreUint64(&pm.rttConfidence, conf)

	rtt := time.Duration(atomic.LoadUint64(&pm.rttEstimate))
	log.Debug("Relaxed PeersManager QoS values", "rtt", rtt, "confidence", float64(conf)/1000000.0, "ttl", pm.requestTTL())
}

// idlePeers retrieves a flat list of all currently idle peers satisfying the
// protocol version constraints, using the provided function to check idleness.
// The resulting set of peers are sorted by their measure throughput.
func (pm *peersManager) idlePeers(idleCheck func(*Peer) bool, lock bool, throughput func(*Peer) float64) ([]*Peer, int) {
	pm.peersLock.RLock()
	defer pm.peersLock.RUnlock()

	idle, total := make([]*Peer, 0, len(pm.peers)), 0
	for _, p := range pm.peers {
		if idleCheck(p) {
			if lock {
				if atomic.CompareAndSwapInt32(&p.idle, 0, 1) {
					idle = append(idle, p)
				}
			} else {
				idle = append(idle, p)
			}
		}
		total++
	}

	for i := 0; i < len(idle); i++ {
		for j := i + 1; j < len(idle); j++ {
			if throughput(idle[i]) < throughput(idle[j]) {
				idle[i], idle[j] = idle[j], idle[i]
			}
		}
	}
	return idle, total
}

// medianRTT returns the median RTT of the peerset, considering only the tuning
// peers if there are more peers available.
func (pm *peersManager) medianRTT() time.Duration {
	// Gather all the currently measured Round trip times
	pm.peersLock.RLock()
	defer pm.peersLock.RUnlock()

	rtts := make([]float64, 0, len(pm.peers))
	for _, p := range pm.peers {
		p.lock.RLock()
		rtts = append(rtts, float64(p.GetRtt()))
		p.lock.RUnlock()
	}
	sort.Float64s(rtts)

	median := RttMaxEstimate
	if qosTuningPeers <= len(rtts) {
		median = time.Duration(rtts[qosTuningPeers/2]) // Median of our tuning peers
	} else if len(rtts) > 0 {
		median = time.Duration(rtts[len(rtts)/2]) // Median of our connected peers (maintain even like this some baseline qos)
	}
	// Restrict the RTT into some QoS defaults, irrelevant of true RTT
	if median < rttMinEstimate {
		median = rttMinEstimate
	}
	if median > RttMaxEstimate {
		median = RttMaxEstimate
	}
	return median
}

// registerPeer injects a peer into peerManager's peer set and init the throughput
// and RTT.

//todo:if use the median throughput init a new registered peer.
func (pm *peersManager) registerPeer(p *Peer) error {
	// Retrieve the current median RTT as a sane default
	p.SetRtt(pm.medianRTT())

	// Register the new peer with some meaningful defaults
	pm.peersLock.Lock()
	if _, ok := pm.peers[p.FP.GetID()]; ok {
		pm.peersLock.Unlock()
		return errAlreadyRegistered
	}
	//if len(pm.peers) > 0 {
	//	p.SetThroughput(0)
	//
	//	for _, peer := range pm.peers {
	//		peer.lock.RLock()
	//		p.SetThroughput(p.GetThroughput() + peer.GetThroughput())
	//		peer.lock.RUnlock()
	//	}
	//
	//	p.SetThroughput(p.GetThroughput() / float64(len(pm.peers)))
	//}

	pm.peers[p.FP.GetID()] = p
	pm.peersLock.Unlock()

	log.Info("PeersManager register peer", "Peer Id", p.FP.GetID(), "Init throughput", p.GetThroughput(), "Init rtt", p.GetRtt(), "Peers' len", len(pm.peers))
	return nil
}

// unRegisterPeer remove the peerManager.
func (pm *peersManager) unRegisterPeer(id string) error {
	pm.peersLock.Lock()
	defer pm.peersLock.Unlock()

	_, ok := pm.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(pm.peers, id)
	log.Info("PeersManager unregister sync peer", "Peer Id", id, "Remain Size", len(pm.peers))
	return nil
}

func (pm *peersManager) len() int {
	pm.peersLock.RLock()
	defer pm.peersLock.RUnlock()
	return len(pm.peers)
}

//initRegisterPeer register peer into peersmanager when peermanager is init.
func (pm *peersManager) initRegisterPeer(dp FetcherPeer) error {
	log.Info("PeersManager init sync peer", "peer", dp.GetID())
	peer := newPeer(dp)
	if err := pm.registerPeer(peer); err != nil {
		log.Error("Failed to register sync peer", "err", err)
		return err
	}
	pm.qosReduceConfidence()
	return nil
}

// RegisterPeer injects a new peer into the set of block source to be
// used for fetching state nodes , blocks and packages from.
func (pm *peersManager) RegisterPeer(dp FetcherPeer) error {
	peer := newPeer(dp)
	if err := pm.registerPeer(peer); err != nil {
		log.Error("Failed to register sync peer", "err", err)
		return err
	}
	pm.qosReduceConfidence()

	//inform the peersmanager a new peer arrived.
	pm.newPeerCh <- struct{}{}
	return nil
}

// UnregisterPeer remove a peer from the known list, preventing any action from
// the specified peer. An effort is also made to return any pending fetches into
// the queue.
func (pm *peersManager) UnregisterPeer(id string) error {
	// Unregister the peer from the active peer set and revoke any fetch tasks

	if err := pm.unRegisterPeer(id); err != nil {
		log.Error("Failed to unregister sync peer", "err", err)
		return err
	}
	return nil
}

// IdlePeers retrieves a flat list of all the currently node-data-idle
// peers within the active peer set, ordered by their reputation.
func (pm *peersManager) IdlePeers(lock bool) ([]*Peer, int) {
	idle := func(p *Peer) bool {
		return p.GetIdle() == 0
	}
	throughput := func(p *Peer) float64 {
		p.lock.RLock()
		defer p.lock.RUnlock()
		return p.GetThroughput()
	}
	return pm.idlePeers(idle, lock, throughput)
}

//func (pm *peersManager) getPeer() []string {
//	var peers []string
//	for id := range pm.peers {
//		peers = append(peers, id)
//	}
//	return peers
//}
