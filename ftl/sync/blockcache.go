package sync

import (
	"errors"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
	"sync"
)

type blockCache struct {
	sync.RWMutex
	cache       map[common.Hash]*types.Block
	clearCycle  int
	greedy      uint8
	remainedLen int
}

func NewBlockCache(clearCycleCount int, greedy uint8, remainedLen int) *blockCache {
	return &blockCache{cache: make(map[common.Hash]*types.Block), clearCycle: clearCycleCount * int(greedy), greedy: greedy, remainedLen: remainedLen}
}

func (b *blockCache) Get(key common.Hash) (*types.Block, bool) {
	b.RLock()
	defer b.RUnlock()
	value, ok := b.cache[key]
	return value, ok
}

func (b *blockCache) Put(key common.Hash, value *types.Block) {
	b.Lock()
	defer b.Unlock()
	b.cache[key] = value
}

func (b *blockCache) Delete(hash common.Hash) error {
	b.Lock()
	defer b.Unlock()
	if _, ok := b.cache[hash]; !ok {
		return errors.New("key not exist")
	}
	delete(b.cache, hash)
	return nil
}
func (b *blockCache) Len() int {
	return len(b.cache)
}
func (b *blockCache) GC(currentHeight uint64) {
	if currentHeight%uint64(b.clearCycle) != 0 {
		return
	}
	keys := b.getExpiredKeys(currentHeight)
	for _, key := range keys {
		b.Delete(key)
	}
	log.Info("do gc for block cache", "currentHeight", currentHeight, "len(expired)", len(keys), "len(cache)", b.Len())
}
func (b *blockCache) isExpire(block *types.Block, currentHeight uint64) bool {
	return block.Header.Height+uint64(b.remainedLen) < currentHeight
}
func (b *blockCache) getExpiredKeys(currentHeight uint64) (keys []common.Hash) {
	b.RLock()
	defer b.RUnlock()
	for key, item := range b.cache {
		if b.isExpire(item, currentHeight) {
			keys = append(keys, key)
		}
	}
	return keys
}
