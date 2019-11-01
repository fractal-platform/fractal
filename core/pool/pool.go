package pool

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/rcrowley/go-metrics"
)

const (
	chainUpdateChanSize = 10              // chainUpdateChanSize is the size of channel listening to ChainUpdateEvent.
	evictionInterval    = time.Minute     // Time interval to check for evictable elements
	statsReportInterval = 8 * time.Second // Time interval to report element pool stats
)

var (
	// ErrReplaceUnderpriced is returned if a element is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement element underpriced")

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas fee (based on gas limit) + value")

	ErrIsNotAPacker = errors.New("this transaction should be sent to a packer")
)

type sender interface {
	sender(ele Element) (common.Address, error) // Sender is used to find the from address of the element.
}

type helper interface {
	reset(pool Pool, block *types.Block)                                           // Invoked when a new block coming, Reset already surrounded by pool's lock.
	validate(pool Pool, ele Element, currentState StateDB, chain BlockChain) error // When add a new element into pool, pool's user can provide some Validate logic.
	sender(ele Element) (common.Address, error)                                    // Sender is used to find the from address of the element.
}

// pool contains all currently known elements. Elements (transactions or transaction packages)
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are stably included in the blockchain.
type pool struct {
	conf                   config.PoolConfig
	chain                  BlockChain
	eleFeed                event.Feed
	scope                  event.SubscriptionScope
	chainUpdateCh          chan types.ChainUpdateEvent
	chainUpdateSub         event.Subscription
	mu                     sync.RWMutex
	currentHeight          uint64
	currentState           *state.StateDB               // Current state in the blockchain head
	pendingState           *state.ManagedState          // Pending state tracking virtual nonces
	locals                 *accountSet                  // Set of local element to exempt from eviction rules
	journal                *eleJournal                  // Journal of local element to back up to disk
	queue                  map[common.Address]*EleList  // Queued and processable elements
	beats                  map[common.Address]time.Time // Last heartbeat from each known account
	all                    *eleLookup                   // All elements to allow lookups
	helper                 helper                       // helper give some hooks
	queuedDiscardCounter   metrics.Counter
	queuedRateLimitCounter metrics.Counter // Dropped due to rate limiting
	invalidTxCounter       metrics.Counter
	wg                     sync.WaitGroup // for shutdown sync
	elemType               reflect.Type
}

// NewPool creates a new element pool to gather, sort and filter inbound
// elements from the network.
func NewPool(conf config.PoolConfig, chain BlockChain, elemType reflect.Type, helper helper) Pool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	conf = (&conf).Sanitize()

	// Create the element pool with its initial settings
	pool := &pool{
		conf:          conf,
		chain:         chain,
		currentHeight: chain.CurrentBlock().Header.Height,
		queue:         make(map[common.Address]*EleList),
		beats:         make(map[common.Address]time.Time),
		all:           newEleLookup(),
		chainUpdateCh: make(chan types.ChainUpdateEvent, chainUpdateChanSize),
		helper:        helper,
		elemType:      elemType,
	}
	pool.locals = newAccountSet(pool.helper)
	for _, addr := range conf.Locals {
		log.Info("Setting new local account", "address", addr)
		pool.locals.add(addr)
	}
	pool.initState(chain.CurrentBlock())

	// If local elements and journaling is enabled, load from disk
	if !conf.NoLocals && conf.Journal != "" {
		pool.journal = newEleJournal(elemType.String()+conf.Journal, elemType)

		if err := pool.journal.load(pool.AddLocals); err != nil {
			log.Warn("Failed to load element journal", "err", err)
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			log.Warn("Failed to rotate element journal", "err", err)
		}
	}
	// Subscribe events from blockchain
	pool.chainUpdateSub = pool.chain.SubscribeChainUpdateEvent(pool.chainUpdateCh)

	// Metrics for pool
	pool.queuedDiscardCounter = metrics.NewRegisteredCounter(elemType.String()+"-pool/queued/discard", nil)
	pool.queuedRateLimitCounter = metrics.NewRegisteredCounter(elemType.String()+"-pool/queued/ratelimit", nil)
	pool.invalidTxCounter = metrics.NewRegisteredCounter(elemType.String()+"-pool/invalid", nil)

	// Start the event loop and return
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the element pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and element
// eviction events.
func (pool *pool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and element eviction tickers
	var prevQueued int

	report := time.NewTicker(statsReportInterval)
	defer report.Stop()

	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()

	journal := time.NewTicker(pool.conf.Rejournal)
	defer journal.Stop()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainUpdateEvent
		case ev := <-pool.chainUpdateCh:
			if ev.Block != nil {
				pool.mu.Lock()
				pool.doReset(ev.Block)
				pool.mu.Unlock()
			}
			// Be unsubscribed due to system stopped
		case <-pool.chainUpdateSub.Err():
			return

			// Handle stats reporting ticks
		case <-report.C:
			pool.mu.RLock()
			queued := pool.stats()
			pool.mu.RUnlock()

			if queued != prevQueued {
				log.Debug("Transaction pool status report", "queued", queued)
				prevQueued = queued
			}

			// Handle inactive account element eviction
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Skip local elements from the eviction mechanism
				if pool.locals.contains(addr) {
					continue
				}
				// Any non-locals old enough should be removed
				if time.Since(pool.beats[addr]) > pool.conf.Lifetime {
					for _, elem := range pool.queue[addr].Flatten() {
						pool.removeEle(elem.Hash())
					}
				}
			}
			pool.mu.Unlock()

			// Handle local element journal rotation
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					log.Warn("Failed to rotate local element journal", "err", err)
				}
				pool.mu.Unlock()
			}
		}
	}
}

func (pool *pool) initState(newHead *types.Block) {
	stateDb, err := pool.chain.StateAt(newHead.Header.StateHash)
	if err != nil {
		log.Error("Failed to init elePool state", "err", err)
		return
	}
	pool.currentState = stateDb
	pool.pendingState = state.ManageState(stateDb, pool.elemType)
}

// doReset retrieves the current state of the blockchain and ensures the content
// of the element pool is valid with regard to the chain state.
func (pool *pool) doReset(newHead *types.Block) {
	// invoke the hook
	pool.helper.reset(pool, newHead)

	// The block chain height increased
	if newHead.Header.Height > pool.currentHeight {
		pool.currentHeight = newHead.Header.Height

		stateDb, err := pool.chain.StateAt(newHead.Header.StateHash)
		if err != nil {
			log.Error("Failed to doReset elempool state", "err", err)
			return
		}
		pool.currentState = stateDb
		pool.pendingState = state.ManageState(stateDb, pool.elemType)

		pool.demoteUnexecutables()

		// Update all accounts to the latest known pending nonce
		for addr, list := range pool.queue {
			elems := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
			pool.pendingState.SetNonce(addr, elems[len(elems)-1].Nonce()+1)
		}
	}

	queue := pool.stats()
	log.Info("Element pool doReset", "queue", queue, "all", len(pool.all.all))
}

// Stop terminates the element pool.
func (pool *pool) Stop() {
	// Unsubscribe all subscriptions registered from elempool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainUpdateSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	log.Info("Element pool stopped")
}

// SubscribeNewElemEvent registers a subscription of NewElemEvent and
// starts sending event to the given channel.
func (pool *pool) SubscribeNewElemEvent(ch chan<- NewElemEvent) event.Subscription {
	return pool.scope.Track(pool.eleFeed.Subscribe(ch))
}

// State returns the virtual managed state of the element pool.
func (pool *pool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

func (pool *pool) StateUnsafe() *state.ManagedState {
	return pool.pendingState
}

// GetNonce returns the nonce of the element pool for the related address.
// If get function is not provided, getNonce return the transaction nonce by default.
func (pool *pool) GetNonce(from common.Address, get func(from common.Address, state *state.ManagedState) uint64) uint64 {
	if get == nil {
		return pool.pendingState.GetNonce(from)
	}
	return get(from, pool.pendingState)
}

// Stats retrieves the current pool stats, namely the number of pending elements.
func (pool *pool) Stats() int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending elements.
func (pool *pool) stats() int {
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return queued
}

// Content retrieves the data content of the element pool, returning all the
// pending elements, grouped by account and sorted by nonce.
func (pool *pool) Content() map[common.Address][]Element {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	queued := make(map[common.Address][]Element)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return queued
}

func (pool *pool) ContentForPack() map[common.Address][]Element {
	return pool.Content()
}

// Locals retrieves the accounts currently considered local by the pool.
func (pool *pool) Locals() []common.Address {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.locals.flatten()
}

// local retrieves all currently known local elements, groupped by origin
// account and sorted by nonce. The returned element set is a copy and can be
// freely modified by calling code.
func (pool *pool) local() map[common.Address][]Element {
	elements := make(map[common.Address][]Element)
	for addr := range pool.locals.accounts {
		if queued := pool.queue[addr]; queued != nil {
			elements[addr] = append(elements[addr], queued.Flatten()...)
		}
	}
	return elements
}

// add validates a element and inserts it into the queue for later execution.
//
// If a newly added element is marked as local, its sending account will be
// whitelisted, preventing any associated element from being dropped out of
// the pool due to pricing constraints.
func (pool *pool) add(ele Element, local bool) (bool, error) {
	// If the element is already known, discard it
	hash := ele.Hash()
	if pool.all.Get(hash) != nil {
		log.Debug("Discarding already known element", "hash", hash)
		return false, fmt.Errorf("known element: %x", hash)
	}
	// If the element fails basic validation, discard it
	if err := pool.validate(ele); err != nil {
		log.Debug("Discarding invalid element", "hash", hash, "err", err)
		pool.invalidTxCounter.Inc(1)
		return false, err
	}

	// If the element pool is full, cut out
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.conf.GlobalQueue {
		pool.cutoutQueueList(queued)
	}

	// Push new element into queue
	replace, err := pool.enqueueEle(hash, ele)
	if err != nil {
		return false, err
	}

	from, _ := pool.helper.sender(ele)
	// Mark local addresses and journal local elements
	if local {
		if !pool.locals.contains(from) {
			log.Info("Setting new local account", "address", from)
			pool.locals.add(from)
		}
	}
	pool.journalEle(from, ele)

	go pool.eleFeed.Send(NewElemEvent{[]Element{ele}})

	log.Debug("Pooled new element", "hash", hash, "from", from)
	return replace, nil
}

func (pool *pool) validate(ele Element) error {
	if pool.helper == nil {
		return nil
	}
	return pool.helper.validate(pool, ele, pool.currentState, pool.chain)
}

// enqueueEle inserts a new element into the pending element queue.
//
// Note, this method assumes the pool lock is held!
func (pool *pool) enqueueEle(hash common.Hash, ele Element) (bool, error) {
	// Try to insert the element into the future queue
	from, _ := pool.helper.sender(ele) // already validated
	if pool.queue[from] == nil {
		pool.queue[from] = newEleList(false)
	}
	inserted, old := pool.queue[from].Add(ele, pool.conf.PriceBump)
	if !inserted {
		// An older element was better, discard this
		pool.queuedDiscardCounter.Inc(1)
		return false, ErrReplaceUnderpriced
	}
	if old != nil {
		pool.all.Remove(old.Hash())
	}
	if pool.all.Get(hash) == nil {
		pool.all.Add(ele)
	}

	pool.beats[from] = time.Now()
	var nonce uint64
	if pool.elemType == reflect.TypeOf(types.TxPackage{}) {
		nonce = pool.pendingState.GetPackageNonce(from)
	} else {
		nonce = pool.pendingState.GetNonce(from)
	}
	if ele.Nonce() >= nonce {
		pool.pendingState.SetNonce(from, ele.Nonce()+1)
	}

	return old != nil, nil
}

// journalEle adds the specified element to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *pool) journalEle(from common.Address, ele Element) {
	// Only journal if it's enabled and the element is local
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(ele); err != nil {
		log.Warn("Failed to journal local element", "err", err)
	}
}

func (pool *pool) AddUnsafe(eles []Element, local bool) []error {
	return pool.addElesUnsafe(eles, local)
}

// AddLocal enqueues a single element into the pool if it is valid, marking
// the Sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *pool) AddLocal(ele Element) error {
	return pool.addEle(ele, !pool.conf.NoLocals)
}

// AddRemote enqueues a single element into the pool if it is valid. If the
// Sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *pool) AddRemote(ele Element) error {
	return pool.addEle(ele, false)
}

// AddLocals enqueues a batch of elements into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *pool) AddLocals(eles []Element) []error {
	return pool.addEles(eles, !pool.conf.NoLocals)
}

// AddRemotes enqueues a batch of elements into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *pool) AddRemotes(eles []Element) []error {
	return pool.addEles(eles, false)
}

// addEle enqueues a single element into the pool if it is valid.
func (pool *pool) addEle(ele Element, local bool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the element and update any state
	_, err := pool.add(ele, local)
	return err
}

// addEles attempts to queue a batch of elements if they are valid.
func (pool *pool) addEles(eles []Element, local bool) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addElesUnsafe(eles, local)
}

// addElesUnsafe attempts to queue a batch of elements if they are valid,
// whilst assuming the element pool lock is already held.
func (pool *pool) addElesUnsafe(eles []Element, local bool) []error {
	// Add the batch of element, tracking the accepted ones
	errs := make([]error, len(eles))

	for i, elem := range eles {
		_, errs[i] = pool.add(elem, local)
	}
	return errs
}

// Get returns a element if it is contained in the pool
// and nil otherwise.
func (pool *pool) Get(hash common.Hash) Element {
	return pool.all.Get(hash)
}

// removeEle removes a single element from the queue.
func (pool *pool) removeEle(hash common.Hash) {
	// Fetch the element we wish to delete
	ele := pool.all.Get(hash)
	if ele == nil {
		return
	}
	addr, _ := pool.helper.sender(ele) // already validated during insertion

	// Remove it from the list of known elements
	pool.all.Remove(hash)

	// Transaction is in the queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(ele)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

func (pool *pool) cutoutQueueList(queued uint64) {
	// Sort all accounts with queued elements by heartbeat
	addresses := make(addressesByHeartbeat, 0, len(pool.queue))
	for addr := range pool.queue {
		if !pool.locals.contains(addr) { // don't drop locals
			addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
		}
	}
	sort.Sort(addresses)

	// Drop elements until the total is below the limit or only locals remain
	for drop := queued - pool.conf.GlobalQueue; drop > 0 && len(addresses) > 0; {
		addr := addresses[len(addresses)-1]
		list := pool.queue[addr.address]

		addresses = addresses[:len(addresses)-1]

		// Drop all elements if they are less than the overflow
		if size := uint64(list.Len()); size <= drop {
			for _, ele := range list.Flatten() {
				pool.removeEle(ele.Hash())
			}
			drop -= size
			pool.queuedRateLimitCounter.Inc(int64(size))
			continue
		}
		// Otherwise drop only last few elements
		elements := list.Flatten()
		for i := len(elements) - 1; i >= 0 && drop > 0; i-- {
			pool.removeEle(elements[i].Hash())
			drop--
			pool.queuedRateLimitCounter.Inc(1)
		}
	}
}

// demoteUnexecutables removes invalid and processed elements from the pools
// pending queue and any subsequent elements that become unexecutable
// are evicted from the queue.
func (pool *pool) demoteUnexecutables() {
	stateBeforeCacheHeight, _, ok := pool.GetStateBeforeCacheHeight()
	if !ok {
		return
	}

	// Iterate over all accounts and traverse all elements
	removed := 0
	for addr, list := range pool.queue {
		var nonce uint64
		if pool.elemType == TxPackageType {
			nonce = stateBeforeCacheHeight.GetPackageNonce(addr)
		} else {
			nonce = stateBeforeCacheHeight.GetNonce(addr)
		}

		log.Debug("demoteUnexecutables", "nonce", nonce, "addr", addr)

		// Drop all elements that are deemed too old (low nonce)
		for _, elem := range list.Forward(nonce) {
			hash := elem.Hash()
			log.Debug("Removed old pending element", "hash", hash, "nonce", elem.Nonce())
			pool.all.Remove(hash)
			removed += 1
		}

		// Drop if the package is too old (does not satisfy the chain rules of tx package entry)
		if pool.elemType == TxPackageType {
			var filter = func(ele Element) bool { // true => drop
				relateBlock := pool.chain.GetBlock(ele.(*types.TxPackage).BlockFullHash())
				if relateBlock == nil {
					return true
				}
				pkgHeight := relateBlock.Header.Height + uint64(pool.chain.GetGreedy()) + params.PackerKeyConfirmDistance
				minHeight, err := pool.chain.MinAvailablePackageHeight()
				if err != nil {
					return true
				}
				if pkgHeight < minHeight {
					return true
				}
				return false
			}
			for _, elem := range list.Filter(filter) {
				hash := elem.Hash()
				log.Debug("Removed too old package", "hash", hash, "nonce", elem.Nonce())
				pool.all.Remove(hash)
				removed += 1
			}
		}

		// Delete the entire queue entry if it became empty.
		if list.Empty() {
			delete(pool.queue, addr)
			delete(pool.beats, addr)
		}
	}
	log.Info("demoteUnexecutables", "type", pool.elemType.Name(), "removed", removed)
}

// GetStateBeforeCacheHeight return the last stable block's state.
func (pool *pool) GetStateBeforeCacheHeight() (*state.StateDB, *types.Block, bool) {
	return pool.chain.GetStateBeforeCacheHeight(pool.chain.CurrentBlock(), uint8(params.ConfirmHeightDistance))
}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addressesByHeartbeat []addressByHeartbeat

func (a addressesByHeartbeat) Len() int           { return len(a) }
func (a addressesByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addressesByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a Sender
// capable of deriving addresses from elements.
type accountSet struct {
	accounts map[common.Address]struct{}
	sender   sender
	cache    *[]common.Address
}

// newAccountSet creates a new address set with an associated Sender derivations.
func newAccountSet(sender sender) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		sender:   sender,
	}
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

// flatten returns the list of addresses within this set, also caching it for later
// reuse. The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		accounts := make([]common.Address, 0, len(as.accounts))
		for account := range as.accounts {
			accounts = append(accounts, account)
		}
		as.cache = &accounts
	}
	return *as.cache
}

// eleLookup is used internally by pool to track elements while allowing lookup without
// mutex contention.
//
// Note, although this type is properly protected against concurrent access, it
// is **not** a type that should ever be mutated or even exposed outside of the
// element pool, since its internal state is tightly coupled with the pools
// internal mechanisms. The sole purpose of the type is to permit out-of-bound
// peeking into the pool in pool.Get without having to acquire the widely scoped
// pool.mu mutex.
type eleLookup struct {
	all  map[common.Hash]Element
	lock sync.RWMutex
}

// newEleLookup returns a new eleLookup structure.
func newEleLookup() *eleLookup {
	return &eleLookup{
		all: make(map[common.Hash]Element),
	}
}

// Get returns a element if it exists in the lookup, or nil if not found.
func (l *eleLookup) Get(hash common.Hash) Element {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return l.all[hash]
}

// Count returns the current number of items in the lookup.
func (l *eleLookup) Count() int {
	l.lock.RLock()
	defer l.lock.RUnlock()

	return len(l.all)
}

// Add adds a element to the lookup.
func (l *eleLookup) Add(ele Element) {
	l.lock.Lock()
	defer l.lock.Unlock()

	l.all[ele.Hash()] = ele
}

// Remove removes a element from the lookup.
func (l *eleLookup) Remove(hash common.Hash) {
	l.lock.Lock()
	defer l.lock.Unlock()

	delete(l.all, hash)
}
