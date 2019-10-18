package downloader

import (
	"errors"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
	"sync"
	"sync/atomic"
	"time"
)

const (
	States = 0
	Blocks = 1
	Pkgs   = 2
)

const (
	measurementImpact = 0.1 // The impact a single measurement has on a peer's final throughput value.
)

var (
	errAlreadyFetching = errors.New("already fetching blocks from peer")
)

// peerDropFn is a callback type for dropping a peer detected as malicious.
type peerDropFn func(id string, addBlack bool)

type dataPack interface {
	PeerId() string
	Items() int
	Stats() string
}

type Peer struct {
	FP         FetcherPeer   // Peer used to fetch data
	started    time.Time     // Time instance when the last node data fetch was started
	throughput float64       // Number of node data pieces measured to be retrievable per second
	idle       int32         // Current node data activity state of the peer (idle = 0, active = 1)
	rtt        time.Duration // Request Round trip time to track responsiveness (QoS)
	lock       sync.RWMutex
}

func newPeer(dp FetcherPeer) *Peer {
	return &Peer{
		FP:   dp,
		idle: 0,
	}
}

// GetIdle return 0 if the peer is idle else return 0
func (p *Peer) GetIdle() int32 {
	p.lock.RLock()
	defer p.lock.RUnlock()

	return p.idle
}

func (p *Peer) GetThroughput() float64 {
	return p.throughput
}

func (p *Peer) SetThroughput(throughput float64) {
	p.throughput = throughput
}

// SetNodeDataIdle sets the peer to idle, allowing it to execute new state trie
// data retrieval requests. Its estimated state retrieval throughput is updated
// with that measured just now.
func (p *Peer) SetIdle(delivered int) {
	p.setIdle(p.started, delivered, &p.throughput, &p.idle)
}

func (p *Peer) SetIdleWithoutDelivered() {
	atomic.StoreInt32(&p.idle, 0)
}

func (p *Peer) GetRtt() time.Duration {
	return p.rtt
}

func (p *Peer) SetRtt(rtt time.Duration) {
	p.rtt = rtt
}

// setIdle sets the peer to idle, allowing it to execute new retrieval requests.
// Its estimated retrieval throughput is updated with that measured just now.
func (p *Peer) setIdle(started time.Time, delivered int, throughput *float64, idle *int32) {
	// Irrelevant of the scaling, make sure the peer ends up idle
	defer atomic.StoreInt32(idle, 0)

	p.lock.Lock()
	defer p.lock.Unlock()

	// If nothing was delivered (hard timeout / unavailable data), reduce throughput to minimum
	if delivered == 0 {
		*throughput = 0
		return
	}
	// Otherwise update the throughput with a new measurement
	elapsed := time.Since(started) + 1 // +1 (ns) to ensure non-zero divisor
	measured := float64(delivered) / (float64(elapsed) / float64(time.Second))

	*throughput = (1-measurementImpact)*(*throughput) + measurementImpact*measured
	p.rtt = time.Duration((1-measurementImpact)*float64(p.rtt) + measurementImpact*float64(elapsed))

	log.Info("Peer throughput measurements updated", "xps", p.throughput, "rtt", p.rtt)
}

// FetchNodeData sends a node state data retrieval request to the remote peer.
func (p *Peer) FetchNodeData(hashes []common.Hash) error {

	// Short circuit if the peer is already fetching
	if !atomic.CompareAndSwapInt32(&p.idle, 0, 1) {
		return errAlreadyFetching
	}
	p.started = time.Now()

	go p.FP.RequestNodeData(hashes)

	return nil
}

// FetchBodies sends blocks range or a block hash retrieval request to the remote peer.
func (p *Peer) FetchBlocks(stage protocol.SyncStage, roundFrom uint64, roundTo uint64) error {
	p.started = time.Now()

	go p.FP.RequestSyncBlocks(stage, roundFrom, roundTo)

	return nil
}

// FetchBodies sends packages retrieval request to the remote peer.
func (p *Peer) FetchPkgs(stage protocol.SyncStage, hashes []common.Hash) error {
	p.started = time.Now()

	go p.FP.RequestSyncPkgs(stage, hashes)
	return nil
}

type FetcherPeer interface {
	GetID() string
	RequestNodeData(hashes []common.Hash) error
	RequestSyncPkgs(stage protocol.SyncStage, hashes []common.Hash) error
	RequestSyncBlocks(stage protocol.SyncStage, roundFrom uint64, roundTo uint64) error
}
