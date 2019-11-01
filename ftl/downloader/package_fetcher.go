package downloader

import (
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// PkgsReq represents a set of packages grouped together
// into a single data retrieval network packet.
type pkgsReq struct {
	fetcherReq
	items    []common.Hash
	response []*types.TxPackage
	index    int64
}

// isTimeout returns if this request timeout.
func (req *pkgsReq) isTimeout() bool {
	return req.response == nil
}

// processPkg return the size of the package and remove the hash of the package from
// req's items.
func (req *pkgsReq) processPkg(txPackage *types.TxPackage) int {
	for i := range req.items {
		if txPackage.Hash() == req.items[i] {
			req.items = append(req.items[:i], req.items[i+1:]...)

			// get length of tx package
			encoded, _ := rlp.EncodeToBytes(txPackage)
			return len(encoded)
		}
	}
	return 0
}

// pkgsFetcher schedules request for fetching packages according a given hashes set.
type pkgsFetcher struct {
	pm     *peersManager
	logger log.Logger

	autoStop bool
	stage    protocol.SyncStage

	// for response receive
	repCh   chan dataPack
	deliver chan *pkgsReq

	// for pkg fetch
	lock       sync.RWMutex
	reqs       []common.Hash
	pending    mapset.Set // pending peer ID set
	trackReq   chan *pkgsReq
	newReq     chan struct{}
	finishReqs chan struct{}
	reqIndex   int64

	done       chan struct{}
	err        error
	finishCh   chan struct{}
	finishOnce sync.Once
}

// newPkgsFetcher creates a new packages scheduler. This method does not
// yet start the sync. The user needs to call run to initiate.
func newPkgsFetcher(peersManager *peersManager, autoStop bool, stage protocol.SyncStage, logger log.Logger) *pkgsFetcher {
	return &pkgsFetcher{
		pm:     peersManager,
		logger: logger,

		autoStop: autoStop,
		stage:    stage,

		repCh:   make(chan dataPack),
		deliver: make(chan *pkgsReq),

		pending:    mapset.NewSet(),
		trackReq:   make(chan *pkgsReq),
		newReq:     make(chan struct{}),
		finishReqs: make(chan struct{}),
		reqIndex:   0,

		done:     make(chan struct{}),
		finishCh: make(chan struct{}),
	}
}

// addReqs add a block request into hashReq set.
func (pf *pkgsFetcher) addReqs(pkgHashes []common.Hash) {
	pf.logger.Info("pkgsFetcher receive a new reqs.", "len", len(pkgHashes))
	if len(pkgHashes) == 0 {
		// indicate the first time
		pf.newReq <- struct{}{}
		return
	}

	if len(pf.reqs) == 0 && pf.pending.Cardinality() == 0 {
		pf.logger.Debug("tell pkgsFetcher receive a new req")
		pf.appendReq(pkgHashes)
		pf.newReq <- struct{}{}
		return
	}
	pf.appendReq(pkgHashes)
}

// deliverData injects a new batch of packages data received from a remote node.
func (pf *pkgsFetcher) deliverData(id string, data []*types.TxPackage) {
	select {
	case pf.repCh <- &pkgsPack{id, data}:
		return
	case <-pf.done:
		return
	}
}

// finish stops the whole fetcher process.
func (pf *pkgsFetcher) finish() {
	pf.finishOnce.Do(func() { close(pf.finishCh) })
}

// loop for request handle
func (pf *pkgsFetcher) runReqHandleLoop() {
	var (
		active   = make(map[string]*pkgsReq)
		finished []*pkgsReq
		timeout  = make(chan *pkgsReq)
	)

	defer func() {
		for _, req := range active {
			req.timer.Stop()
			req.peer.SetIdle(len(req.items))
		}
	}()

	for {
		var (
			deliverReq   *pkgsReq
			deliverReqCh chan *pkgsReq
		)
		if len(finished) > 0 {
			deliverReq = finished[0]
			deliverReqCh = pf.deliver
		}

		select {
		case <-pf.done:
			return

		case deliverReqCh <- deliverReq:
			// Shift out the first request, but also set the emptied slot to nil for GC
			copy(finished, finished[1:])
			finished[len(finished)-1] = nil
			finished = finished[:len(finished)-1]

		case pack := <-pf.repCh:
			// Handle incoming pkg pack
			pf.logger.Info("Receive pkgs", "len", pack.Items())

			// Discard any data not requested (or previously timed out)
			req := active[pack.PeerId()]
			if req == nil {
				pf.logger.Error("Ignore unrequested pkgs", "peer", pack.PeerId(), "len", pack.Items())
				continue
			}

			// Finalize the request and queue up for processing
			req.timer.Stop()
			req.response = pack.(*pkgsPack).pkgs

			finished = append(finished, req)
			delete(active, pack.PeerId())

			// Handle timed-out requests:
		case req := <-timeout:
			// If the peer is already requesting something else, ignore the stale timeout.
			// This can happen when the timeout and the delivery happens simultaneously,
			// causing both pathways to trigger.
			if active[req.peer.FP.GetID()] != req {
				continue
			}
			pf.logger.Info("Fetch package timeout", "peer", req.peer.FP.GetID(), "len", len(req.items), "timeout", req.timeout.Seconds())

			// Move the timed out data back into the download queue
			finished = append(finished, req)
			delete(active, req.peer.FP.GetID())

			// Track outgoing pkg requests:
		case req := <-pf.trackReq:
			// If an active request already exists for this peer, we have a problem. In
			// theory the trie node schedule must never assign two requests to the same
			// peer. In practice however, a peer might receive a request, disconnect and
			// immediately reconnect before the previous times out. In this case the first
			// request is never honored, alas we must not silently overwrite it, as that
			// causes valid requests to go missing and sync to get stuck.
			if old := active[req.peer.FP.GetID()]; old != nil {
				pf.logger.Warn("Busy peer assigned new pkgs fetching", "peer", req.peer.FP.GetID())

				// Make sure the previous one doesn't get siletly lost
				old.timer.Stop()
				old.dropped = true

				finished = append(finished, old)
			}
			// Start a timer to notify the sync loop if the peer stalled.
			req.timer = time.AfterFunc(req.timeout, func() {
				select {
				case timeout <- req:
				case <-pf.done:
					// Prevent leaking of timer goroutines in the unlikely case where a
					// timer is fired just before exiting runStateSync.
				}
			})
			active[req.peer.FP.GetID()] = req
		}
	}
}

//loop deal with the req from peers or the error signature.
func (pf *pkgsFetcher) runReqAssignLoop() {
	defer close(pf.done)
	timer := time.NewTimer(60 * time.Second)
	for {
		pf.logger.Info("Sync packages loop is still running", "remainSize", len(pf.reqs), "pendingReq", pf.pending.Cardinality())

		if pf.pm.len() == 0 && pf.autoStop {
			pf.logger.Info("Package fetcher stopped, no avail peer")
			pf.err = errNoAvailPeer
			pf.finishReqs <- struct{}{}
			return
		}

		// Tasks assigned, Wait for something to happen
		pf.assignTasks()

		if pf.pending.Cardinality() == 0 && len(pf.reqs) == 0 {
			pf.logger.Debug("pkgsfetcer:  not has reqs!")
			pf.finishReqs <- struct{}{}
		}

		select {
		case <-timer.C:
			// continue
			timer.Reset(60 * time.Second)

		case <-pf.newReq:
			//New packages request massage arrive.

		case <-pf.finishCh:
			pf.logger.Info("pkg fetcher finished")
			return

		case req := <-pf.deliver:
			pf.logger.Info("Received pkgs response", "peer", req.peer.FP.GetID(), "index", req.index, "count", len(req.response), "dropped", req.dropped, "timeout", req.isTimeout())
			if req.dropped {
				pf.logger.Info("This pkg request is dropped", "peer", req.peer.FP.GetID(), "index", req.index, "count", len(req.items))

				pf.lock.Lock()
				pf.reqs = append(pf.reqs, req.items...)
				pf.pending.Remove(req.peer.FP.GetID())
				pf.lock.Unlock()
				continue
			}

			delivered := pf.process(req)
			req.peer.SetIdle(delivered)
		}
	}
}

//assignTasks finds the peers which are idle and assign task according to their cap
//which cap was calculated by their performance(ttl and throughput) in last request.
func (pf *pkgsFetcher) assignTasks() {
	//get peers which are idle
	peers, _ := pf.pm.IdlePeers(true)
	pf.logger.Info("Package fetcher assign tasks", "idles", len(peers))

	for i, p := range peers {
		// Assign a batch of fetches proportional to the estimated latency/bandwidth
		cap := pf.pkgsCapacity(p)
		req := &pkgsReq{fetcherReq: fetcherReq{
			peer:    p,
			timeout: pf.pm.requestTTL(),
			dropped: false,
		},
			index: atomic.AddInt64(&pf.reqIndex, 1)}

		// if all request has been assigned and there are some peer are idle
		// inform pkgFetcher to fetching remaining pkgs.
		if pf.fillTasks(cap, req) {
			for _, idlePeer := range peers[i:] {
				idlePeer.SetIdleWithoutDelivered()
			}
			return
		}
		// If the peer was assigned tasks to fetch, send the network request
		if len(req.items) > 0 {
			pf.lock.Lock()
			pf.pending.Add(req.peer.FP.GetID())
			pf.lock.Unlock()

			select {
			case pf.trackReq <- req:
				err := req.peer.FetchPkgs(pf.stage, req.items)
				if err != nil {
					pf.logger.Error("Failed fetch pkgs", "error", err)
				}
			case <-pf.done:
				return
			}
		}
	}
}

//fillTask assign task into req.When array request is not empty,fillTask assign
// tasks in request array.Otherwise assign tasks which are not assigned according the cap of peers.
func (pf *pkgsFetcher) fillTasks(n int, req *pkgsReq) bool {
	// Refill available tasks from the scheduler.
	pf.lock.Lock()
	defer pf.lock.Unlock()
	if len(pf.reqs) == 0 {
		return true
	}
	if n <= len(pf.reqs) {
		req.items = append(req.items, pf.reqs[:n]...)
		pf.reqs = pf.reqs[n:]
	} else {
		req.items = append(req.items, pf.reqs...)
		pf.reqs = []common.Hash{}
	}

	pf.logger.Info("Assign a new pkgs request", "index", req.index, "size", len(req.items), "peer", req.peer.FP.GetID())
	return false
}

//process checkout whether the req was timeout.If the req was timeout or dropped, if so
//push then into request array.Then for each response in req, process it and calculate the size of successfull data.
func (pf *pkgsFetcher) process(req *pkgsReq) int {
	successful := 0
	pf.lock.Lock()
	defer pf.lock.Unlock()

	// If the req is timeout, injects it into hash requests set and remove the peer of the req.
	if req.isTimeout() {
		pf.logger.Info("This pkg request is timeout", "peer", req.peer.FP.GetID(), "count", len(req.items))
		pf.reqs = append(pf.reqs, req.items...)
		if pf.pm.dropPeer != nil {
			pf.pm.UnregisterPeer(req.peer.FP.GetID())
			pf.pm.dropPeer(req.peer.FP.GetID(), false)
		} else {
			pf.logger.Warn("unregistered a fail peer but not drop it", "peer", req.peer.FP.GetID())
		}
		pf.pending.Remove(req.peer.FP.GetID())
		return 0
	}

	for _, pkg := range req.response {
		pkgSize := req.processPkg(pkg)
		successful += pkgSize
	}
	if len(req.items) > 0 {
		pf.logger.Warn("Response is not full", "loss", len(req.items), "peer", req.peer.FP.GetID())
		pf.reqs = append(pf.reqs, req.items...)
	}
	pf.pending.Remove(req.peer.FP.GetID())

	return successful
}

// pkgsCapacity return the predicted amount of packages the peer should fetcher next time.
func (pf *pkgsFetcher) pkgsCapacity(peer *Peer) int {
	peer.lock.RLock()
	defer peer.lock.RUnlock()

	return int(math.Min(math.Max(float64(MinPkgsFetch), peer.GetThroughput()/float64(BytesPerPkg)*float64(pf.pm.requestRTT())/float64(time.Second)), float64(MaxPkgsFetch)))
}

func (pf *pkgsFetcher) appendReq(reqHashes []common.Hash) {
	pf.lock.Lock()
	defer pf.lock.Unlock()
	pf.reqs = append(pf.reqs, reqHashes...)
}

//pkgsPack is a batch of packages returned by a peer.
type pkgsPack struct {
	peerID string
	pkgs   []*types.TxPackage
}

func (pp *pkgsPack) PeerId() string { return pp.peerID }
func (pp *pkgsPack) Items() int     { return len(pp.pkgs) }
func (pp *pkgsPack) Stats() string  { return fmt.Sprintf("%d", len(pp.pkgs)) }
