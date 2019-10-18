package types

import (
	"errors"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/hashicorp/golang-lru"
)

var ErrMapNotFoundInCache = errors.New("map not found in cache")

type PackerECPubKey [65]byte

type PackerInfo struct {
	PackerPubKey PackerECPubKey
	Coinbase     common.Address
	RpcAddress   string // include ip address and port,
}

type PackerInfoMap struct {
	IndexPackerMap map[uint32]*PackerInfo
	PubKeyIndexMap map[PackerECPubKey]uint32
}

func NewPackerInfoMap() *PackerInfoMap {
	m := &PackerInfoMap{
		IndexPackerMap: make(map[uint32]*PackerInfo),
		PubKeyIndexMap: make(map[PackerECPubKey]uint32),
	}
	return m
}

type PackerInfoMapCache struct {
	cache *lru.Cache

	mu sync.RWMutex
}

func NewPackerInfoMapCache(cacheSize uint8) (*PackerInfoMapCache, error) {
	cache, err := lru.New(int(cacheSize))
	if err != nil {
		return nil, err
	}
	m := &PackerInfoMapCache{
		cache: cache,
	}
	return m, nil
}

func (ps *PackerInfoMapCache) Get(blockHash common.Hash) (*PackerInfoMap, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if p, ok := ps.cache.Get(blockHash); ok {
		return p.(*PackerInfoMap), nil
	}

	return nil, ErrMapNotFoundInCache
}

func (ps *PackerInfoMapCache) Put(blockHash common.Hash, packerInfoMap *PackerInfoMap) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	ps.cache.Add(blockHash, packerInfoMap)
}
