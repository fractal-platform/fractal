package pool

import (
	"container/heap"
	"math/big"
	"sort"

	"github.com/fractal-platform/fractal/utils/log"
)

// nonceHeap is a heap.Interface implementation over 64bit unsigned integers for
// retrieving sorted elements from the possibly gapped future queue.
type nonceHeap []uint64

func (h nonceHeap) Len() int           { return len(h) }
func (h nonceHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h nonceHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *nonceHeap) Push(x interface{}) {
	*h = append(*h, x.(uint64))
}

func (h *nonceHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

// eleSortedMap is a nonce->element hash map with a heap based index to allow
// iterating over the contents in a nonce-incrementing way.
type eleSortedMap struct {
	items map[uint64]Element // Hash map storing the element data
	index *nonceHeap         // Heap of nonces of all the stored elements (non-strict mode)
	cache []Element          // Cache of the elements already sorted
}

// newEleSortedMap creates a new nonce-sorted element map.
func newEleSortedMap() *eleSortedMap {
	return &eleSortedMap{
		items: make(map[uint64]Element),
		index: new(nonceHeap),
	}
}

// Get retrieves the current elements associated with the given nonce.
func (m *eleSortedMap) Get(nonce uint64) Element {
	return m.items[nonce]
}

// Put inserts a new element into the map, also updating the map's nonce
// index. If an element already exists with the same nonce, it's overwritten.
func (m *eleSortedMap) Put(ele Element) {
	nonce := ele.Nonce()
	if m.items[nonce] == nil {
		heap.Push(m.index, nonce)
	}
	m.items[nonce], m.cache = ele, nil
}

// Forward removes all elements from the map with a nonce lower than the
// provided threshold. Every removed element is returned for any post-removal
// maintenance.
func (m *eleSortedMap) Forward(threshold uint64) []Element {
	var removed []Element

	// Pop off heap items until the threshold is reached
	for m.index.Len() > 0 && (*m.index)[0] < threshold {
		nonce := heap.Pop(m.index).(uint64)
		removed = append(removed, m.items[nonce])
		delete(m.items, nonce)
	}
	// If we had a cached order, shift the front
	if m.cache != nil {
		m.cache = m.cache[len(removed):]
	}
	return removed
}

// Filter iterates over the list of elements and removes all of them for which
// the specified function evaluates to true.
func (m *eleSortedMap) Filter(filter func(Element) bool) []Element {
	var removed []Element

	// Collect all the elements to filter out
	for nonce, ele := range m.items {
		if filter(ele) {
			removed = append(removed, ele)
			delete(m.items, nonce)
		}
	}
	// If elements were removed, the heap and cache are ruined
	if len(removed) > 0 {
		*m.index = make([]uint64, 0, len(m.items))
		for nonce := range m.items {
			*m.index = append(*m.index, nonce)
		}
		heap.Init(m.index)

		m.cache = nil
	}
	return removed
}

// Cap places a hard limit on the number of items, returning all elements
// exceeding that limit.
func (m *eleSortedMap) Cap(threshold int) []Element {
	// Short circuit if the number of items is under the limit
	if len(m.items) <= threshold {
		return nil
	}
	// Otherwise gather and drop the highest nonce'd elements
	var drops []Element

	sort.Sort(*m.index)
	for size := len(m.items); size > threshold; size-- {
		drops = append(drops, m.items[(*m.index)[size-1]])
		delete(m.items, (*m.index)[size-1])
	}
	*m.index = (*m.index)[:threshold]
	heap.Init(m.index)

	// If we had a cache, shift the back
	if m.cache != nil {
		m.cache = m.cache[:len(m.cache)-len(drops)]
	}
	return drops
}

// Remove deletes a element from the maintained map, returning whether the
// element was found.
func (m *eleSortedMap) Remove(nonce uint64) bool {
	// Short circuit if no element is present
	_, ok := m.items[nonce]
	if !ok {
		return false
	}
	// Otherwise delete the element and fix the heap index
	for i := 0; i < m.index.Len(); i++ {
		if (*m.index)[i] == nonce {
			heap.Remove(m.index, i)
			break
		}
	}
	delete(m.items, nonce)
	m.cache = nil

	return true
}

// Len returns the length of the element map.
func (m *eleSortedMap) Len() int {
	return len(m.items)
}

// Flatten creates a nonce-sorted slice of elements based on the loosely
// sorted internal representation. The result of the sorting is cached in case
// it's requested again before any modifications are made to the contents.
func (m *eleSortedMap) Flatten() []Element {
	// If the sorting was not cached yet, create and cache it
	if m.cache == nil {
		m.cache = make([]Element, 0, len(m.items))
		for _, ele := range m.items {
			m.cache = append(m.cache, ele)
		}
		sort.Sort(EleByNonce(m.cache))
	}
	// Copy the cache to prevent accidental modifications
	elements := make([]Element, len(m.cache))
	copy(elements, m.cache)
	return elements
}

// EleList is a "list" of elements belonging to an account, sorted by account
// nonce. The same type can be used both for storing contiguous elements for
// the queue; and for storing gapped elements for the queue, with minor behavioral changes.
type EleList struct {
	strict   bool          // Whether nonces are strictly continuous or not
	elements *eleSortedMap // Heap indexed sorted hash map of the elements
}

// newEleList create a new element list for maintaining nonce-indexable fast,
// gapped, sortable element lists.
func newEleList(strict bool) *EleList {
	return &EleList{
		strict:   strict,
		elements: newEleSortedMap(),
	}
}

// Overlaps returns whether the element specified has the same nonce as one
// already contained within the list.
func (l *EleList) Overlaps(ele Element) bool {
	return l.elements.Get(ele.Nonce()) != nil
}

// Add tries to insert a new element into the list, returning whether the
// element was accepted, and if yes, any previous element it replaced.
func (l *EleList) Add(ele Element, priceBump uint64) (bool, Element) {
	// If there's an older better element, abort
	old := l.elements.Get(ele.Nonce())
	if old != nil {
		threshold := new(big.Int).Div(new(big.Int).Mul(old.GasPrice(), big.NewInt(100+int64(priceBump))), big.NewInt(100))
		// Have to ensure that the new gas price is higher than the old gas
		// price as well as checking the percentage threshold to ensure that
		// this is accurate for low (nFra-level) gas price replacements
		if old.GasPrice().Cmp(ele.GasPrice()) >= 0 || threshold.Cmp(ele.GasPrice()) > 0 {
			return false, nil
		}
		log.Info("Add an element, replace the old one.", "old hash", old.Hash(), "new hash", ele.Hash())
	}

	l.elements.Put(ele)
	return true, old
}

// Forward removes all elements from the list with a nonce lower than the
// provided threshold. Every removed element is returned for any post-removal
// maintenance.
func (l *EleList) Forward(threshold uint64) []Element {
	return l.elements.Forward(threshold)
}

// Filter iterates over the list of elements and removes all of them for which
// the specified function evaluates to true.
func (l *EleList) Filter(filter func(Element) bool) []Element {
	return l.elements.Filter(filter)
}

// Cap places a hard limit on the number of items, returning all elements
// exceeding that limit.
func (l *EleList) Cap(threshold int) []Element {
	return l.elements.Cap(threshold)
}

// Remove deletes a element from the maintained list, returning whether the
// element was found, and also returning any element invalidated due to
// the deletion (strict mode only).
func (l *EleList) Remove(ele Element) (bool, []Element) {
	// Remove the element from the set
	nonce := ele.Nonce()
	if removed := l.elements.Remove(nonce); !removed {
		return false, nil
	}
	return true, nil
}

// Len returns the length of the element list.
func (l *EleList) Len() int {
	return l.elements.Len()
}

// Empty returns whether the list of elements is empty or not.
func (l *EleList) Empty() bool {
	return l.Len() == 0
}

// Flatten creates a nonce-sorted slice of elements based on the loosely
// sorted internal representation. The result of the sorting is cached in case
// it's requested again before any modifications are made to the contents.
func (l *EleList) Flatten() []Element {
	return l.elements.Flatten()
}
