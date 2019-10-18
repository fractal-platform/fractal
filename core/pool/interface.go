package pool

import (
	"container/heap"
	"math/big"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/event"
)

type EleByNonce []Element

func (s EleByNonce) Len() int           { return len(s) }
func (s EleByNonce) Less(i, j int) bool { return s[i].Nonce() < s[j].Nonce() }
func (s EleByNonce) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

type ElementWithFrom struct {
	Element
	From common.Address
}
type EleByPrice []ElementWithFrom

func (s EleByPrice) Len() int           { return len(s) }
func (s EleByPrice) Less(i, j int) bool { return s[i].GasPrice().Cmp(s[j].GasPrice()) > 0 } // We want Pop to give us the highest gas price one, not lowest, priority so we use greater than here.
func (s EleByPrice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *EleByPrice) Push(x interface{}) {
	*s = append(*s, x.(ElementWithFrom))
}

func (s *EleByPrice) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

type ElementsByPriceAndNonce struct {
	eles  map[common.Address][]Element // Per account nonce-sorted list of elements
	heads EleByPrice                   // Next element for each unique account (price heap)
}

// NewElementsByPriceAndNonce creates a element set that can retrieve
// price sorted elements in a nonce-honouring way.
//
// Note, the input map is reowned so the caller should not interact any more with
// it after providing it to the constructor.
func NewElementsByPriceAndNonce(eles map[common.Address][]Element) *ElementsByPriceAndNonce {
	// Initialize a price based heap with the head elements
	heads := make(EleByPrice, 0, len(eles))

	for from, accEles := range eles {
		heads = append(heads, ElementWithFrom{accEles[0], from})
		eles[from] = accEles[1:]
	}
	heap.Init(&heads)

	// Assemble and return the element set
	return &ElementsByPriceAndNonce{
		eles:  eles,
		heads: heads,
	}
}

// Peek returns the next element by price.
func (t *ElementsByPriceAndNonce) Peek() ElementWithFrom {
	if len(t.heads) == 0 {
		return ElementWithFrom{}
	}
	return t.heads[0]
}

// Shift replaces the current best head with the next one from the same account.
func (t *ElementsByPriceAndNonce) Shift() {
	addr := t.heads[0].From

	if eles, ok := t.eles[addr]; ok && len(eles) > 0 {
		t.heads[0], t.eles[addr] = ElementWithFrom{eles[0], addr}, eles[1:]
		heap.Fix(&t.heads, 0)
	} else {
		heap.Pop(&t.heads)
	}
}

type Element interface {
	Nonce() uint64
	Hash() common.Hash
	GasPrice() *big.Int
}

type NewElemEvent struct {
	Ems []Element
}

type Helper interface {
	Reset(pool Pool, block *types.Block)                                           // Invoked when a new block coming, Reset already surrounded by pool's lock.
	Validate(pool Pool, ele Element, currentState StateDB, chain BlockChain) error // When add a new element into pool, pool's user can provide some Validate logic.
	Sender(ele Element) (common.Address, error)                                    // Sender is used to find the from address of the element.
}

type Pool interface {
	// AddRemotes enqueues a batch of elements into the pool if they are valid.
	// If the senders are not among the locally tracked ones, full pricing constraints
	// will apply.
	AddRemotes(eles []Element) []error

	// AddLocals enqueues a batch of elements into the pool if they are valid,
	// marking the senders as a local ones in the mean time, ensuring they go around
	// the local pricing constraints.
	AddLocals(eles []Element) []error

	// AddRemote enqueues a single element into the pool if it is valid. If the
	// Sender is not among the locally tracked ones, full pricing constraints will
	// apply.
	AddRemote(ele Element) error

	// AddLocal enqueues a single element into the pool if it is valid, marking
	// the Sender as a local one in the mean time, ensuring it goes around the local
	// pricing constraints.
	AddLocal(ele Element) error

	// AddUnsafe enqueues a batch of elements into the pool if they are valid.
	// But AddUnsafe is NOT thread safe.
	AddUnsafe(eles []Element, local bool) []error

	// Get returns a element if it is contained in the pool
	// and nil otherwise.
	Get(hash common.Hash) Element

	// Locals retrieves the accounts currently considered local by the pool.
	Locals() []common.Address

	// Content retrieves the data content of the element pool, returning all the
	// pending elements, grouped by account and sorted by nonce.
	Content() map[common.Address][]Element
	ContentForPack() map[common.Address][]Element

	// Stats retrieves the current pool stats, namely the number of pending elements.
	Stats() int

	// GetNonce returns the nonce of the element pool for the related address.
	// If get function is not provided, getNonce return the transaction nonce by default.
	GetNonce(from common.Address, get func(from common.Address, state *state.ManagedState) uint64) uint64

	// StateUnsafe return the virtual managed state of the element pool.
	// But it is NOT thread safe.
	StateUnsafe() *state.ManagedState

	// GetStateBeforeCacheHeight return the last stable block's state.
	GetStateBeforeCacheHeight() (*state.StateDB, *types.Block, bool)

	// SubscribeNewElemEvent registers a subscription of NewElemEvent and
	// starts sending event to the given channel.
	SubscribeNewElemEvent(ch chan<- NewElemEvent) event.Subscription

	// Stop terminates the element pool.
	Stop()
}

type BlockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash) *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
	GetStateBeforeCacheHeight(block *types.Block, cacheHeight uint8) (*state.StateDB, *types.Block, bool)
	SubscribeChainUpdateEvent(ch chan<- types.ChainUpdateEvent) event.Subscription
	MinAvailablePackageHeight() (uint64, error)
	GetChainID() uint64
	GetGreedy() uint8
}

type StateDB interface {
	GetBalance(addr common.Address) *big.Int
}
