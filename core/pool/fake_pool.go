package pool

import (
	"github.com/fractal-platform/fractal/core/types"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/utils/log"
)

type fakePool struct {
	// transaction pool for all address
	pool map[common.Address][]Element
	all  *eleLookup

	// mutex
	mu sync.RWMutex

	eleFeed event.Feed
	scope   event.SubscriptionScope
	helper  Helper

	startCleanTime     int64
	cleanPeriod        int64
	leftEleNumEachAddr int
}

// NewFakePool creates a fake transaction pool
func NewFakePool(startCleanTime int64, cleanPeriod int64, leftEleNumEachAddr int, helper Helper) Pool {
	// Create the transaction pool with its initial settings
	pool := &fakePool{
		pool:   make(map[common.Address][]Element),
		all:    newEleLookup(),
		helper: helper,

		startCleanTime:     startCleanTime,
		cleanPeriod:        cleanPeriod,
		leftEleNumEachAddr: leftEleNumEachAddr,
	}
	log.Info("Init fake pool")

	if pool.startCleanTime > 0 {
		timer := time.NewTimer(time.Duration(pool.startCleanTime) * time.Millisecond)
		go func(t *time.Timer) {
			<-t.C
			pool.cleanPool()
		}(timer)
	}

	return pool
}

func (pool *fakePool) cleanPool() {
	ticker := time.NewTicker(time.Duration(pool.cleanPeriod) * time.Millisecond)
	go func(t *time.Ticker) {
		for {
			<-t.C

			pool.mu.Lock()
			cleanNum := 0
			for addr, oldList := range pool.pool {
				eleLength := len(oldList)
				if eleLength <= pool.leftEleNumEachAddr {
					continue
				}

				// need to remove (eleLength - pool.leftEleNumEachAddr)
				for i := 0; i < eleLength-pool.leftEleNumEachAddr; i++ {
					hash := oldList[i].Hash()
					if pool.all.Get(hash) != nil {
						pool.all.Remove(hash)
					}
				}
				pool.pool[addr] = oldList[eleLength-pool.leftEleNumEachAddr:]
				cleanNum += eleLength - pool.leftEleNumEachAddr
			}
			log.Info("fake pool cleanPool", "clean number", cleanNum)
			pool.mu.Unlock()
		}
	}(ticker)
}

func (pool *fakePool) AddUnsafe(eles []Element, local bool) []error {
	pool.addElesUnsafe(eles)
	return nil
}

func (pool *fakePool) AddLocal(ele Element) error {
	pool.addEle(ele)
	return nil
}

func (pool *fakePool) AddLocals(eles []Element) []error {
	pool.addEles(eles)
	return nil
}

func (pool *fakePool) AddRemote(ele Element) error {
	pool.addEle(ele)
	return nil
}

func (pool *fakePool) AddRemotes(eles []Element) []error {
	pool.addEles(eles)
	return nil
}

func (pool *fakePool) StateUnsafe() *state.ManagedState {
	panic("fake pool doesn't need this")
}

func (pool *fakePool) Get(hash common.Hash) Element {
	return pool.all.Get(hash)
}

func (pool *fakePool) Locals() []common.Address {
	return nil
}

func (pool *fakePool) Content() map[common.Address][]Element {
	return pool.pool
}

func (pool *fakePool) ContentForPack() map[common.Address][]Element {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	content := pool.pool

	// clean up
	pool.pool = make(map[common.Address][]Element)
	pool.all = newEleLookup()

	return content
}

func (p *fakePool) Stats() int {
	count := 0
	for _, elems := range p.pool {
		count = count + len(elems)
	}
	return count
}

func (p *fakePool) GetNonce(from common.Address, get func(from common.Address, state *state.ManagedState) uint64) uint64 {
	panic("use userSet nonce")
}

func (*fakePool) GetStateBeforeCacheHeight() (*state.StateDB, *types.Block, bool) {
	panic("fake pool doesn't need this")
}

func (p *fakePool) SubscribeNewElemEvent(ch chan<- NewElemEvent) event.Subscription {
	return p.scope.Track(p.eleFeed.Subscribe(ch))
}

func (*fakePool) Stop() {
}

func (pool *fakePool) addEle(ele Element) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.add(ele)
}

func (pool *fakePool) addEles(eles []Element) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.addElesUnsafe(eles)
}

func (pool *fakePool) addElesUnsafe(eles []Element) {
	for _, elem := range eles {
		pool.add(elem)
	}
}

func (pool *fakePool) add(ele Element) {
	hash := ele.Hash()
	if pool.all.Get(hash) != nil {
		return
	}
	from, _ := pool.helper.Sender(ele)

	pool.pool[from] = append(pool.pool[from], ele)
	pool.all.Add(ele)

	go pool.eleFeed.Send(NewElemEvent{[]Element{ele}})
}
