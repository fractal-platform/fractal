package downloader

import (
	"fmt"
	"hash"
	"math"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/crypto/sha3"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/trie"
	"github.com/fractal-platform/fractal/utils/log"
)

// StateSync schedules requests for downloading a particular state trie defined
// by a given state root.
type StateSync struct {
	fetcher
	trackReq         chan *stateReq
	sched            *trie.Sync                 // State trie sync scheduler defining the tasks
	keccak           hash.Hash                  // Keccak256 hasher to verify deliveries with
	tasks            map[common.Hash]*stateTask // Set of tasks currently queued for retrieval
	numUncommitted   int
	bytesUncommitted int
	deliver          chan *stateReq // Delivery channel multiplexing peer responses
	syncStatsLock    sync.RWMutex
	syncStatsState   stateSyncStats

	chaindb dbwrapper.Database
}

// syncState starts downloading state with the given root Hash and peers.
func SyncState(peers map[string]FetcherPeer, dropPeerFn peerDropFn, root common.Hash, chaindb dbwrapper.Database) *StateSync {
	s := newStateSync(peers, dropPeerFn, root, chaindb)
	go s.runStateSync()
	return s
}

// deliverData injects a new batch of node state data received from a remote node.
func (s *StateSync) DeliverData(id string, data [][]byte) (err error) {
	return s.fetcher.deliver(&statePack{id, data})
}

// runStateSync runs a state synchronisaion until it completes
func (s *StateSync) runStateSync() {
	var (
		active   = make(map[string]*stateReq) // Currently in-flight requests
		finished []*stateReq                  // Completed or failed requests
		timeout  = make(chan *stateReq)       // Timed out active requests
	)
	defer func() {
		// Cancel active request timers on exit. Also set peers to idle so they're
		// available for the next sync.
		for _, req := range active {
			req.timer.Stop()
			req.peer.SetIdle(len(req.items))
		}
	}()
	// Run the state sync.
	go s.run()
	defer s.Cancel()

	for {
		// Enable sending of the first buffered element if there is one.
		var (
			deliverReq   *stateReq
			deliverReqCh chan *stateReq
		)
		if len(finished) > 0 {
			deliverReq = finished[0]
			deliverReqCh = s.deliver
		}

		// The StateSync lifecycle:
		select {

		case <-s.done:
			return

			// Send the next finished request to the current sync:
		case deliverReqCh <- deliverReq:
			// Shift out the first request, but also set the emptied slot to nil for GC
			copy(finished, finished[1:])
			finished[len(finished)-1] = nil
			finished = finished[:len(finished)-1]

			// Handle incoming state packs:
		case pack := <-s.repCh:
			// Discard any data not requested (or previously timed out)

			//log.Info("Recieve Node Date from peersManager","pack",pack)
			req := active[pack.PeerId()]
			if req == nil {
				log.Debug("Unrequested node data", "peer", pack.PeerId(), "len", pack.Items())
				continue
			}
			// Finalize the request and queue up for processing
			req.timer.Stop()
			req.response = pack.(*statePack).states

			finished = append(finished, req)
			delete(active, pack.PeerId())
			//
			//	// Handle unregister peer connections:
			//case p := <-s.unregisterCh:
			//	// Skip if no request is currently pending
			//	req := active[p]
			//	if req == nil {
			//		continue
			//	}
			//	// Finalize the request and queue up for processing
			//	req.timer.Stop()
			//	req.dropped = true
			//
			//	finished = append(finished, req)
			//	delete(active, p)

			// Handle timed-out requests:
		case req := <-timeout:
			// If the peer is already requesting something else, ignore the stale timeout.
			// This can happen when the timeout and the delivery happens simultaneously,
			// causing both pathways to trigger.
			if active[req.peer.FP.GetID()] != req {
				continue
			}
			// Move the timed out data back into the download queue
			finished = append(finished, req)
			delete(active, req.peer.FP.GetID())

			// Track outgoing state requests:
		case req := <-s.trackReq:
			// If an active request already exists for this peer, we have a problem. In
			// theory the trie node schedule must never assign two requests to the same
			// peer. In practice however, a peer might receive a request, disconnect and
			// immediately reconnect before the previous times out. In this case the first
			// request is never honored, alas we must not silently overwrite it, as that
			// causes valid requests to go missing and sync to get stuck.
			if old := active[req.peer.FP.GetID()]; old != nil {
				log.Warn("Busy peer assigned new state fetch", "peer", req.peer.FP.GetID())

				// Make sure the previous one doesn't get siletly lost
				old.timer.Stop()
				old.dropped = true

				finished = append(finished, old)
			}
			// Start a timer to notify the sync loop if the peer stalled.
			req.timer = time.AfterFunc(req.timeout, func() {
				select {
				case timeout <- req:
				case <-s.done:
					// Prevent leaking of timer goroutines in the unlikely case where a
					// timer is fired just before exiting runStateSync.
				}
			})
			active[req.peer.FP.GetID()] = req
		}
	}
}

// stateTask represents a single trie node download task, containing a set of
// peers already attempted retrieval from to detect stalled syncs and abort.
type stateTask struct {
	attempts map[string]struct{}
}

// stateReq represents a batch of state fetch requests grouped together into
// a single data retrieval network packet.
type stateReq struct {
	fetcherReq
	items    []common.Hash              // Hashes of the state items to download
	tasks    map[common.Hash]*stateTask // Download tasks to track previous attempts
	response [][]byte                   // Response data of the peer (nil for timeouts)
}

// timedOut returns if this request timed out.
func (req *stateReq) timeOut() bool {
	return req.response == nil
}

// stateSyncStats is a collection of progress stats to report during a state trie
// sync to RPC requests as well as to display in user logs.
type stateSyncStats struct {
	processed  uint64 // Number of state entries processed
	duplicate  uint64 // Number of state entries downloaded twice
	unexpected uint64 // Number of non-requested state entries received
	pending    uint64 // Number of still pending state entries
}

// newStateSync creates a new state trie download scheduler. This method does not
// yet start the sync. The user needs to call run to initiate.
func newStateSync(peers map[string]FetcherPeer, dropPeerFn peerDropFn, root common.Hash, chaindb dbwrapper.Database) *StateSync {
	peersManager := newPeersManager(dropPeerFn)

	for _, p := range peers {
		peersManager.initRegisterPeer(p)
	}

	return &StateSync{
		fetcher: fetcher{
			peers:  peersManager,
			cancel: make(chan struct{}),
			repCh:  make(chan dataPack),
			done:   make(chan struct{}),
		},
		sched:    state.NewStateSync(root, chaindb),
		keccak:   sha3.NewKeccak256(),
		tasks:    make(map[common.Hash]*stateTask),
		deliver:  make(chan *stateReq),
		trackReq: make(chan *stateReq),
		chaindb:  chaindb,
	}
}

// run starts the task assignment and response processing loop, blocking until
// it finishes, and finally notifying any goroutines waiting for the loop to
// finish.
func (s *StateSync) run() {
	s.err = s.loop()
	close(s.done)
}

// loop is the main event loop of a state trie sync. It it responsible for the
// assignment of new tasks to peers (including sending it to them) as well as
// for the processing of inbound data. Note, that the loop does not directly
// receive data from peers, rather those are buffered up in the downloader and
// pushed here async. The reason is to decouple processing from data receipt
// and timeouts.
func (s *StateSync) loop() (err error) {

	defer func() {
		cerr := s.commit(true)
		if err == nil {
			err = cerr
		}
	}()

	// Keep assigning new tasks until the sync completes or aborts
	for s.sched.Pending() > 0 {
		if err = s.commit(false); err != nil {
			return err
		}
		// Tasks assigned, Wait for something to happen
		s.assignTasks()

		if s.peers.len() == 0 {
			close(s.cancel)
		}
		select {
		case <-s.peers.newPeerCh:
			// New peer arrived, try to assign it download tasks

		case <-s.cancel:
			return errCancelStateFetch

		case req := <-s.deliver:
			// Response, disconnect or timeout triggered, drop the peer if stalling
			log.Info("Received node data response", "peer", req.peer.FP.GetID(), "count", len(req.response), "dropped", req.dropped, "timeout", !req.dropped && req.timeOut())
			if len(req.items) <= 2 && !req.dropped && req.timeOut() {
				// 2 items are the minimum requested, if even that times out, we've no use of
				// this peer at the moment.
				log.Warn("Stalling state sync, dropping peer", "peer", req.peer.FP.GetID())
				if s.peers.dropPeer == nil {
					// The dropPeer method is nil when `--copydb` is used for a local copy.
					// Timeouts can occur if e.g. compaction hits at the wrong time, and can be ignored
					log.Warn("peersManager wants to drop peer, but peerdrop-function is not set", "peer", req.peer.FP.GetID())
				} else {
					log.Info("peersManager drop peer", "peer", req.peer.FP.GetID())
					s.peers.UnregisterPeer(req.peer.FP.GetID())
					s.peers.dropPeer(req.peer.FP.GetID(), false)
				}
			}
			// Process all the received blobs and check for stale delivery
			delivered, err := s.process(req)
			if err != nil {
				log.Warn("Node data write error", "err", err)
				return err
			}
			req.peer.SetIdle(delivered)
		}
	}
	return nil
}

// process iterates over a batch of delivered state data, injecting each item
// into a running state sync, re-queuing any items that were requested but not
// delivered. Returns whether the peer actually managed to deliver anything of
// value, and any error that occurred.
func (s *StateSync) process(req *stateReq) (int, error) {
	// Collect processing stats and update progress if valid data was received
	duplicate, unexpected, successful := 0, 0, 0

	defer func(start time.Time) {
		if duplicate > 0 || unexpected > 0 {
			s.updateStats(0, duplicate, unexpected, time.Since(start))
		}
	}(time.Now())

	// Iterate over all the delivered data and inject one-by-one into the trie
	for _, blob := range req.response {
		_, hash, err := s.processNodeData(blob)
		switch err {
		case nil:
			s.numUncommitted++
			s.bytesUncommitted += len(blob)
			successful++
		case trie.ErrNotRequested:
			unexpected++
		case trie.ErrAlreadyProcessed:
			duplicate++
		default:
			return successful, fmt.Errorf("invalid state node %s: %v", hash.TerminalString(), err)
		}
		delete(req.tasks, hash)
	}
	// Put unfulfilled tasks back into the retry queue
	npeers := s.peers.len()
	for hash, task := range req.tasks {
		// If the node did deliver something, missing items may be due to a protocol
		// limit or a previous timeout + delayed delivery. Both cases should permit
		// the node to retry the missing items (to avoid single-peer stalls).
		if len(req.response) > 0 || req.timeOut() {
			delete(task.attempts, req.peer.FP.GetID())
		}
		// If we've requested the node too many times already, it may be a malicious
		// sync where nobody has the right data. Abort.
		if len(task.attempts) >= npeers {
			return successful, fmt.Errorf("state node %s failed with all peers (%d tries, %d peers)", hash, len(task.attempts), npeers)
		}
		// Missing item, place into the retry queue.
		s.tasks[hash] = task
	}
	return successful, nil
}

func (s *StateSync) commit(force bool) error {
	if !force && s.bytesUncommitted < dbwrapper.IdealBatchSize {
		return nil
	}
	start := time.Now()
	b := s.chaindb.NewBatch()
	if written, err := s.sched.Commit(b); written == 0 || err != nil {
		return err
	}
	if err := b.Write(); err != nil {
		return fmt.Errorf("DB write error: %v", err)
	}
	s.updateStats(s.numUncommitted, 0, 0, time.Since(start))
	s.numUncommitted = 0
	s.bytesUncommitted = 0
	return nil
}

// assignTasks attempts to assign new tasks to all idle peers, either from the
// batch currently being retried, or fetching new data from the trie sync itself.
func (s *StateSync) assignTasks() {
	// Iterate over all idle peers and try to assign them state fetches

	peers, _ := s.peers.IdlePeers(false)
	log.Debug("Peer is blocks idle", "peers", peers)

	for _, p := range peers {
		// Assign a batch of fetches proportional to the estimated latency/bandwidth
		cap := s.statesCapacity(p)
		//cap := 384
		req := &stateReq{fetcherReq: fetcherReq{
			peer:    p,
			timeout: s.peers.requestTTL(),
		}}
		s.fillTasks(cap, req)

		// If the peer was assigned tasks to fetch, send the network request
		if len(req.items) > 0 {
			log.Debug("Requesting new batch of data", "type", "state", "count", len(req.items))
			select {
			case s.trackReq <- req:
				log.Info("state fetch node data")
				err := req.peer.FetchNodeData(req.items)
				if err != nil {
					log.Error("Failed fetch node data", "error", err)
				}
				log.Info("finish fetch data")
			case <-s.cancel:
			}
		}
	}
}

// updateStats bumps the various state sync progress counters and displays a log
// message for the user to see.
func (s *StateSync) updateStats(written, duplicate, unexpected int, duration time.Duration) {
	s.syncStatsLock.Lock()
	defer s.syncStatsLock.Unlock()

	s.syncStatsState.pending = uint64(s.sched.Pending())
	s.syncStatsState.processed += uint64(written)
	s.syncStatsState.duplicate += uint64(duplicate)
	s.syncStatsState.unexpected += uint64(unexpected)

	if written > 0 || duplicate > 0 || unexpected > 0 {
		log.Debug("Imported new state entries", "count", written, "elapsed", common.PrettyDuration(duration), "processed", s.syncStatsState.processed, "pending", s.syncStatsState.pending, "retry", len(s.tasks), "duplicate", s.syncStatsState.duplicate, "unexpected", s.syncStatsState.unexpected)
	}
}

// processNodeData tries to inject a trie node data blob delivered from a remote
// peer into the state trie, returning whether anything useful was written or any
// error occurred.
func (s *StateSync) processNodeData(blob []byte) (bool, common.Hash, error) {
	res := trie.SyncResult{Data: blob}
	s.keccak.Reset()
	s.keccak.Write(blob)
	s.keccak.Sum(res.Hash[:0])
	committed, _, err := s.sched.Process([]trie.SyncResult{res})
	return committed, res.Hash, err
}

// fillTasks fills the given request object with a maximum of n state download
// tasks to send to the remote peer.
func (s *StateSync) fillTasks(n int, req *stateReq) {
	// Refill available tasks from the scheduler.
	if len(s.tasks) < n {
		new := s.sched.Missing(n - len(s.tasks))
		for _, hash := range new {
			s.tasks[hash] = &stateTask{make(map[string]struct{})}
		}
	}
	// Find tasks that haven't been tried with the request's peer.
	req.items = make([]common.Hash, 0, n)
	req.tasks = make(map[common.Hash]*stateTask, n)
	for hash, t := range s.tasks {
		// Stop when we've gathered enough requests
		if len(req.items) == n {
			break
		}
		// Skip any requests we've already tried from this peer
		if _, ok := t.attempts[req.peer.FP.GetID()]; ok {
			continue
		}
		// Assign the request to this peer
		t.attempts[req.peer.FP.GetID()] = struct{}{}
		req.items = append(req.items, hash)
		req.tasks[hash] = t
		delete(s.tasks, hash)
	}
}

// statesCapacity return the predicted amount of nodes the peer should fetcher next time.
func (s *StateSync) statesCapacity(peer *Peer) int {
	peer.lock.RLock()
	defer peer.lock.RUnlock()

	return int(math.Min(1+math.Max(float64(MinStateFetch), peer.GetThroughput()*float64(s.peers.requestRTT())/float64(time.Second)), float64(MaxStateFetch)))
}

// statePack is a batch of states returned by a peer.
type statePack struct {
	peerID string
	states [][]byte
}

func (sp *statePack) PeerId() string { return sp.peerID }
func (sp *statePack) Items() int     { return len(sp.states) }
func (sp *statePack) Stats() string  { return fmt.Sprintf("%d", len(sp.states)) }
