package downloader

import (
	"errors"
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

// blockReqByHash represents a range rounds of blocks grouped together or a certain block
// into a single data retrieval network packet.
type blockReqByHash struct {
	fetcherReq
	items     []common.Hash
	response  []*types.Block
	index     int64
	isTimeout bool
}

// emptyResponse return true,when the response of the req is empty.
func (req *blockReqByHash) emptyResponse() bool {
	return len(req.response) == 0
}

// BlockFetcherByRound schedules requests for fetching blocks according a given rounds range.
type BlockFetcherByHash struct {
	pm     *peersManager
	chain  blockchain
	logger log.Logger

	autoStop bool
	stage    protocol.SyncStage

	// for response receive
	repCh   chan dataPack
	deliver chan *blockReqByHash // Delivery channel multiplexing peer responses

	// for block fetch
	reqs         []common.Hash
	trackReq     chan *blockReqByHash
	newReq       chan bool
	prioReqs     []common.Hash
	assignTaskCh chan struct{}
	reqIndex     int64

	// for pkg fetch
	pkgsFetcher *pkgsFetcher
	pkgReqsCh   chan []common.Hash

	// for block check
	checkBlocks chan struct{}
	pendingLock sync.RWMutex
	pending     map[common.Hash]*types.Block
	outputBlock chan *types.Block

	done       chan struct{}
	err        error
	finishOnce sync.Once
	finishCh   chan struct{}
}

// newBlocksFetcherByHash creates a new blocks scheduler. This method does not
// yet start the sync. The user needs to call run to initiate.
func newBlocksFetcherByHash(reqs []common.Hash, chain blockchain, manager *peersManager, autoStop bool, stage protocol.SyncStage, blockCh chan *types.Block, logger log.Logger) *BlockFetcherByHash {
	pkgsFetcher := newPkgsFetcher(manager, autoStop, stage, logger)
	blockFetcher := &BlockFetcherByHash{
		chain:  chain,
		pm:     manager,
		logger: logger,

		stage:    stage,
		autoStop: autoStop,

		repCh:   make(chan dataPack),
		deliver: make(chan *blockReqByHash),

		reqs:         reqs,
		trackReq:     make(chan *blockReqByHash),
		newReq:       make(chan bool),
		assignTaskCh: make(chan struct{}),
		reqIndex:     0,

		pkgsFetcher: pkgsFetcher,
		pkgReqsCh:   make(chan []common.Hash),

		checkBlocks: make(chan struct{}),
		pending:     make(map[common.Hash]*types.Block),
		outputBlock: blockCh,

		done:     make(chan struct{}),
		finishCh: make(chan struct{}),
	}
	return blockFetcher
}

func (bf *BlockFetcherByHash) DeliverData(id string, data interface{}, kind int) error {
	switch kind {
	case Blocks:
		bf.deliverData(id, data.(types.Blocks))
		return nil
	case Pkgs:
		bf.pkgsFetcher.deliverData(id, data.([]*types.TxPackage))
		return nil
	default:
		return errors.New("wrong kind of deliver data type")
	}
}

// deliverData injects a new batch of blocks data received from a remote node.
func (bf *BlockFetcherByHash) deliverData(id string, data []*types.Block) {
	select {
	case bf.repCh <- &blockPack{id, data}:
		return
	case <-bf.done:
		return
	}
}

func (bf *BlockFetcherByHash) Register(peer FetcherPeer) error {
	err := bf.pm.RegisterPeer(peer)
	if err != nil {
		return err
	}
	return nil
}

func (bf *BlockFetcherByHash) Finish() {
	bf.pkgsFetcher.finish()
	bf.finishOnce.Do(func() { close(bf.finishCh) })
}

// Wait blocks until the fetcher is done or canceled.
func (bf *BlockFetcherByHash) Wait() error {
	<-bf.done
	bf.logger.Info("block fetcher wait returns", "err", bf.err)
	return bf.err
}

func (bf *BlockFetcherByHash) start() {
	// for communication with pkg fetcher
	go bf.deliverPkgReqs()
	go bf.pkgReqFinished()
	go bf.checkFinishBlocks()

	//
	go bf.pkgsFetcher.runReqHandleLoop()
	go bf.pkgsFetcher.runReqAssignLoop()

	go bf.runReqHandleLoop()
	go bf.runReqAssignLoop()
}

// loop for request handle
func (bf *BlockFetcherByHash) runReqHandleLoop() {
	var (
		active   = make(map[string]*blockReqByHash) // Currently in-flight requests
		finished []*blockReqByHash                  // Completed or failed requests
		timeout  = make(chan *blockReqByHash)       // Timed out active requests
	)
	defer func() {
		// Cancel active request timers on exit. Also set peers to idle so they're
		// available for the next sync.
		for _, req := range active {
			req.timer.Stop()
			req.peer.SetIdle(len(req.items))
		}
	}()

	// Run the blocks fetching.
	for {
		// Enable sending of the first buffered element if there is one.
		var (
			deliverReq   *blockReqByHash
			deliverReqCh chan *blockReqByHash
		)
		if len(finished) > 0 {
			deliverReq = finished[0]
			deliverReqCh = bf.deliver
		}

		// The block Sync lifecycle:
		select {
		case <-bf.done:
			return

			// Send the next finished request to the current sync:
		case deliverReqCh <- deliverReq:
			// Shift out the first request, but also set the emptied slot to nil for GC
			copy(finished, finished[1:])
			finished[len(finished)-1] = nil
			finished = finished[:len(finished)-1]

			// Handle incoming block packs:
		case pack := <-bf.repCh:
			// Discard any data not requested (or previously timed out)
			req := active[pack.PeerId()]
			if req == nil {
				bf.logger.Info("Ignore unrequested blocks", "peer", pack.PeerId(), "len", pack.Items())
				continue
			}
			// Finalize the request and queue up for processing
			req.timer.Stop()
			req.response = pack.(*blockPack).blocks

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
			bf.logger.Info("Fetch block timeout", "peer", req.peer.FP.GetID(), "item", len(req.items), "index", req.index)

			// Move the timed out data back into the download queue
			req.isTimeout = true
			finished = append(finished, req)
			delete(active, req.peer.FP.GetID())

			// Track outgoing block requests:
		case req := <-bf.trackReq:
			// If an active request already exists for this peer, we have a problem. In
			// theory the trie node schedule must never assign two requests to the same
			// peer. In practice however, a peer might receive a request, disconnect and
			// immediately reconnect before the previous times out. In this case the first
			// request is never honored, alas we must not silently overwrite it, as that
			// causes valid requests to go missing and sync to get stuck.
			if old := active[req.peer.FP.GetID()]; old != nil {
				bf.logger.Info("Busy peer assigned new blocks fetching", "peer", req.peer.FP.GetID())

				// Make sure the previous one doesn't get silently lost
				old.timer.Stop()
				old.dropped = true

				finished = append(finished, old)
			}

			// Start a timer to notify the sync loop if the peer stalled.
			req.timer = time.AfterFunc(req.timeout, func() {
				select {
				case timeout <- req:
				case <-bf.done:
					// Prevent leaking of timer goroutines in the unlikely case where a
					// timer is fired just before exiting runBlockFetch.
				}
			})
			active[req.peer.FP.GetID()] = req
		}
	}
}

// loop for request assign
func (bf *BlockFetcherByHash) runReqAssignLoop() {
	//
	defer close(bf.done)

	for {
		bf.logger.Info("Sync block loop is still running", "remain", len(bf.reqs), "prioReq", len(bf.prioReqs), "peers", bf.pm.len())

		if bf.pm.len() == 0 && bf.autoStop {
			bf.logger.Info("BlockFetcherByRound No have available peer.")
			bf.err = errNoAvailPeer
			return
		}

		// Tasks assigned, Wait for something to happen
		select {
		case <-bf.assignTaskCh:
			// Tasks assigned,when the reqs in all pkgs requests have been distributed.
			bf.assignTasks()

		case <-bf.pm.newPeerCh:
			//When new peer massage arrive, informs pkgsFetcher to try fetching pgks
			bf.pkgReqsCh <- []common.Hash{}

		case <-bf.newReq:
			//When new block request massage arrive, informs pkgsFetcher to try fetching pkgs
			bf.pkgReqsCh <- []common.Hash{}

		case <-bf.finishCh:
			bf.logger.Info("block fetcher finished")
			return

		case req := <-bf.deliver:
			bf.logger.Info("Received blocks response", "peer", req.peer.FP.GetID(), "count", len(req.response), "dropped", req.dropped, "emptyResponse", req.emptyResponse(), "timeout", req.isTimeout, "reqs", len(req.items), "index", req.index)

			// If the req is dropped, injects it into hash requests set.
			if req.dropped {
				bf.logger.Info("This block request is dropped", "req", req)
				bf.insertRequest(req)
				continue
			}

			// Process all the received blobs and check for stale delivery
			delivered, err := bf.process(req)
			if err != nil {
				bf.logger.Warn("Blocks write error", "err", err)
				bf.err = err
				continue
			}
			req.peer.SetIdle(delivered)

		}
	}
}

//assignTasks finds the peers which are idle and assign task according to their cap
//which cap was calculated by their performance(ttl and throughput) in last request.
func (bf *BlockFetcherByHash) assignTasks() {
	//if all request has been assigned return
	if len(bf.prioReqs) == 0 && len(bf.reqs) == 0 {
		return
	}

	//get peers which are idle
	peers, _ := bf.pm.IdlePeers(true)
	bf.logger.Info("block fetcher assign tasks", "peers", len(peers))

	for i, p := range peers {
		// Assign a batch of fetches proportional to the estimated latency/bandwidth
		cap := bf.blocksCapacity(p)
		req := &blockReqByHash{fetcherReq: fetcherReq{
			peer:    p,
			timeout: bf.pm.requestTTL(),
			dropped: false,
		}, isTimeout: false,
			index: atomic.AddInt64(&bf.reqIndex, 1)}

		if bf.fillTasks(cap, req) {
			for _, idlePeer := range peers[i:] {
				idlePeer.SetIdleWithoutDelivered()
			}
			return
		}

		// If the peer was assigned tasks to fetch, send the network request
		if len(req.items) > 0 {
			select {
			case bf.trackReq <- req:
				err := req.peer.FetchBlocks(bf.stage, req.items, 0, 0)
				if err != nil {
					bf.logger.Error("Failed fetch block", "error", err)
				}
			case <-bf.done:
			}
		}
	}
}

//fillTask assign task into req.When array request is not empty,fillTask assign
// tasks in request array.Otherwise assign tasks which are not assigned according the cap of peers.
func (bf *BlockFetcherByHash) fillTasks(n int, req *blockReqByHash) bool {
	// If the all reqs have been distributed, return true.
	if len(bf.prioReqs) == 0 && len(bf.reqs) == 0 {
		bf.logger.Info("All blocks have been requested!")
		return true
	}

	// If the set of priority request is not empty , deal these request secondly according the
	// cap of peer.
	if len(bf.prioReqs) != 0 {
		bf.logger.Debug("Now the length of prioReq is:", "len", len(bf.prioReqs))
		if len(bf.prioReqs) > n {
			req.items = append(req.items, bf.prioReqs[:n]...)
			bf.prioReqs = bf.prioReqs[n:]
		} else if len(bf.prioReqs) == n {
			req.items = append(req.items, bf.prioReqs[:n]...)
			bf.prioReqs = []common.Hash{}
		} else {
			req.items = append(req.items, bf.prioReqs...)
			bf.prioReqs = []common.Hash{}

			if len(bf.reqs) > n-len(bf.prioReqs) {
				req.items = append(req.items, bf.reqs[:n-len(bf.prioReqs)]...)
				bf.reqs = bf.reqs[n-len(bf.prioReqs):]
			} else {
				req.items = append(req.items, bf.reqs...)
				bf.reqs = []common.Hash{}
			}

		}
	} else {
		if len(bf.reqs) > n {
			req.items = append(req.items, bf.reqs[:n]...)
			bf.reqs = bf.reqs[n:]
		} else {
			req.items = append(req.items, bf.reqs...)
			bf.reqs = []common.Hash{}
		}
	}

	bf.logger.Info("Assign a new blocks Req", "items", len(req.items), "index", req.index, "peer", req.peer.FP.GetID())
	return false
}

//process checkout whether the req was timeout.If the req was timeout or dropped, if so
//push then into request array.Then for each response in req,process it and calculate the size of successful data.If received
//a block which is not in right Round range.Push the req into array of request and return.
func (bf *BlockFetcherByHash) process(req *blockReqByHash) (int, error) {
	successful := 0

	// If the req is timeout, injects it into priority requests set.
	if req.emptyResponse() {
		bf.logger.Info("This req is timeout or response is empty", "timeout", req.isTimeout, "peer", req.peer.FP.GetID(), "items", len(req.items), "index", req.index)

		bf.insertRequest(req)
		//If the req is timeout and the req is not dropped, drop and unregister the peer of the req.
		if bf.pm.dropPeer == nil {
			bf.logger.Warn("peersManager wants to drop peer, but peerdrop-function is not set", "peer", req.peer.FP.GetID())
		} else {
			bf.logger.Info("peersManager drop peer", "peer", req.peer.FP.GetID())
			bf.pm.UnregisterPeer(req.peer.FP.GetID())
			//if two cursors are running, the first cursor connection failed, the second no need to drop the peer
			bf.pm.dropPeer(req.peer.FP.GetID(), false)
		}
		return 0, nil
	}
	if len(req.response) == 0 {
		return successful, nil
	}

	var pkgHashSet = mapset.NewSet()
	var pkgHashSlice []common.Hash

	for _, blob := range req.response {
	ForAgain:
		for i, blockHash := range req.items {
			///If the response in your reqs, process it.
			if blockHash == blob.FullHash() {
				if len(req.items) > 1 {
					req.items = append(req.items[:i], req.items[i+1:]...)
				} else {
					req.items = nil
				}
				hashes, err, blockSize := bf.processBlock(blob, req)
				switch err {
				case nil:
					successful += blockSize
					for _, hash := range hashes {
						if !pkgHashSet.Contains(hash) {
							pkgHashSlice = append(pkgHashSlice, hash)
						}
						pkgHashSet.Add(hash)
					}
					break ForAgain
				case errBlockWithWrongRound:
					bf.logger.Error("Received block not in appropriate Round range", "peer", req.peer.FP.GetID(), "Hash", blob.FullHash(), "Round", blob.Header.Round, "roundFrom", "items", len(req.items), "Height", blob.Header.Height)
					bf.insertRequest(req)
					bf.pm.UnregisterPeer(req.peer.FP.GetID())
					if bf.pm.dropPeer != nil {
						bf.pm.UnregisterPeer(req.peer.FP.GetID())
						bf.pm.dropPeer(req.peer.FP.GetID(), false)
					} else {
						bf.logger.Warn("unregistered a fail peer but not drop it", "peer", req.peer.FP.GetID())
					}
					return 0, nil
				default:
					bf.insertRequest(req)
					bf.pm.UnregisterPeer(req.peer.FP.GetID())
					if bf.pm.dropPeer != nil {
						bf.pm.UnregisterPeer(req.peer.FP.GetID())
						bf.pm.dropPeer(req.peer.FP.GetID(), false)
					} else {
						bf.logger.Warn("unregisted a fail peer but not drop it", "peer", req.peer.FP.GetID())
					}
					return 0, fmt.Errorf("invalid block %s: %v", blob.FullHash().TerminalString(), err)
				}
			}
		}
	}

	if len(req.items) > 0 {
		bf.insertRequest(req)
	}
	bf.logger.Debug("BlockFetcherByRound: add request to pkgsfetcher", "length", len(pkgHashSlice))
	// Send the packages hashes in this reqs to pkgFetcher.
	bf.pkgReqsCh <- pkgHashSlice

	return successful, nil
}

// processBlock checkout if the block in the appropriate Round range,return the packages hashes in blocks
// and add the block into pending set.
func (bf *BlockFetcherByHash) processBlock(b *types.Block, req *blockReqByHash) ([]common.Hash, error, int) {
	bf.logger.Debug("block sync process block", "hash", b.FullHash(), "round", b.Header.Round, "height", b.Header.Height, "pkgsSize", len(b.Body.TxPackageHashes))

	bf.pendingLock.Lock()
	bf.pending[b.FullHash()] = b
	bf.pendingLock.Unlock()
	var pkgsHashes []common.Hash
	for _, hash := range b.Body.TxPackageHashes {
		if !bf.chain.HasTxPackage(hash) && !bf.chain.IsTxPackageInFuture(hash) {
			pkgsHashes = append(pkgsHashes, hash)
		}
	}

	return pkgsHashes, nil, getBlockBytes(b)
}

// insertRequest injects a req into priority request set.
func (bf *BlockFetcherByHash) insertRequest(req *blockReqByHash) {
	bf.prioReqs = append(bf.prioReqs, req.items...)
}

// blocksCapacity return the predicted amount of blocks the peer should fetcher next time.
func (bf *BlockFetcherByHash) blocksCapacity(peer *Peer) int {
	peer.lock.RLock()
	defer peer.lock.RUnlock()

	return int(math.Min(math.Max(float64(MinBlockFetch), (peer.GetThroughput()/float64(BytesPerBlock))*float64(bf.pm.requestRTT())/float64(time.Second)), float64(MaxBlockFetch)))
}

//TODO:pending blocks time out! If malicious player send err-blocks ?

// checkFinishBlocks checkout if the blocks in pending set have received all the packages they have, and send the finished
// block to synchroniser.
func (bf *BlockFetcherByHash) checkFinishBlocks() {
	for {
		select {
		case <-bf.checkBlocks:
			bf.pendingLock.Lock()
			for _, block := range bf.pending {
				if bf.checkFinishBlock(block) {
					bf.logger.Info("Send the block to sync", "hash", block.FullHash(), "round", block.Header.Round, "height", block.Header.Height)
					go func(b *types.Block) {
						bf.outputBlock <- b
					}(block)
					delete(bf.pending, block.FullHash())
				}
			}
			bf.pendingLock.Unlock()
			bf.logger.Info("Now the pending block", "size", len(bf.pending))
		case <-bf.finishCh:
			return

		case <-bf.done:
			return
		}

	}
}

// checkFinishBlock checkout if the packages blong to a block have been insert into
// local database.
func (bf *BlockFetcherByHash) checkFinishBlock(block *types.Block) bool {
	for _, hash := range block.Body.TxPackageHashes {
		has := bf.chain.HasTxPackage(hash) || bf.chain.IsTxPackageInFuture(hash)
		if !has {
			return false
		}
	}
	return true
}

func (bf *BlockFetcherByHash) deliverPkgReqs() {
	for {
		select {
		case pkgsReq := <-bf.pkgReqsCh:
			go bf.pkgsFetcher.addReqs(pkgsReq)

		case <-bf.done:
			return
		}
	}
}

func (bf *BlockFetcherByHash) pkgReqFinished() {
	for {
		select {
		case <-bf.pkgsFetcher.finishReqs:
			bf.logger.Info("BlocksWithPkgsFetcher receive a finished flag!")
			bf.assignTaskCh <- struct{}{}
			bf.checkBlocks <- struct{}{}
		case <-bf.done:
			return
		}
	}
}

// getBlockBytes return the
func getBlockBytes(b *types.Block) int {
	encoded, _ := rlp.EncodeToBytes(b)
	return len(encoded)
}

// blockPack is a batch of blocks returned by a peer.
type blockPack struct {
	peerID string
	blocks []*types.Block
}

func (bp *blockPack) PeerId() string { return bp.peerID }
func (bp *blockPack) Items() int     { return len(bp.blocks) }
func (bp *blockPack) Stats() string  { return fmt.Sprintf("%d", len(bp.blocks)) }
