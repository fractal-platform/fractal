package api

import (
	"context"
	"reflect"
	"sync"

	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/hashicorp/golang-lru"
)

type CheckPointAPI struct {
	ftl              fractal
	checkPointPriKey crypto.PrivateKey

	queryCheckPointCache   *lru.Cache // treePointhash -> checkPoint lru map
	queryCheckPointCacheMu sync.Mutex

	queryCheckPointHashCache   *lru.Cache // treePointhash -> checkPointHash lru map
	queryCheckPointHashCacheMu sync.Mutex
}

func NewCheckPointAPI(ftl fractal, checkPointPriKey crypto.PrivateKey) *CheckPointAPI {
	c := new(CheckPointAPI)
	c.ftl = ftl
	c.checkPointPriKey = checkPointPriKey

	var err error
	c.queryCheckPointCache, err = lru.New(10)
	if err != nil {
		log.Error("NewCheckPointAPI: init queryCheckPointCache error", "err", err)
		return nil
	}

	c.queryCheckPointHashCache, err = lru.New(10)
	if err != nil {
		log.Error("NewCheckPointAPI: init queryCheckPointHashCache error", "err", err)
		return nil
	}

	return c
}

func (c *CheckPointAPI) GetCheckPointHashByIndex(ctx context.Context, index uint64) (*types.SignedCheckPointHash, error) {
	c.queryCheckPointHashCacheMu.Lock()
	defer c.queryCheckPointHashCacheMu.Unlock()

	treePoint, err := c.ftl.BlockChain().GetCheckPointByIndex(index)
	if err != nil {
		return nil, err
	}
	treePointHash := treePoint.Hash()

	if p, ok := c.queryCheckPointHashCache.Get(treePointHash); ok {
		return p.(*types.SignedCheckPointHash), nil
	}

	ret := &types.SignedCheckPointHash{
		Hash: treePointHash,
	}

	key := c.checkPointPriKey
	if key == nil || reflect.ValueOf(key).IsNil() {
		return ret, nil
	}

	sign, err := key.Sign(treePointHash[:])
	if err != nil {
		return ret, nil
	}

	ret.Sign = sign
	c.queryCheckPointHashCache.Add(treePointHash, ret)

	return ret, nil
}

func (c *CheckPointAPI) GetCheckPointByIndex(ctx context.Context, index uint64) (*types.SignedCheckPoint, error) {
	c.queryCheckPointCacheMu.Lock()
	defer c.queryCheckPointCacheMu.Unlock()

	treePoint, err := c.ftl.BlockChain().GetCheckPointByIndex(index)
	if err != nil {
		return nil, err
	}

	if p, ok := c.queryCheckPointCache.Get(treePoint.Hash()); ok {
		return p.(*types.SignedCheckPoint), nil
	}

	ret := &types.SignedCheckPoint{
		CheckPoint: treePoint,
	}

	key := c.checkPointPriKey
	if key == nil || reflect.ValueOf(key).IsNil() {
		return ret, nil
	}

	hash := treePoint.Hash()
	sign, err := key.Sign(hash[:])
	if err != nil {
		return ret, nil
	}

	ret.Sign = sign
	c.queryCheckPointCache.Add(hash, ret)

	return ret, nil
}

func (c *CheckPointAPI) GetLastCheckPointHash(ctx context.Context) *types.SignedCheckPointHash {
	c.queryCheckPointHashCacheMu.Lock()
	defer c.queryCheckPointHashCacheMu.Unlock()

	treePointHash := c.ftl.BlockChain().GetCheckPoint().Hash()

	if p, ok := c.queryCheckPointHashCache.Get(treePointHash); ok {
		return p.(*types.SignedCheckPointHash)
	}

	ret := &types.SignedCheckPointHash{
		Hash: treePointHash,
	}

	key := c.checkPointPriKey
	if key == nil || reflect.ValueOf(key).IsNil() {
		return ret
	}

	sign, err := key.Sign(treePointHash[:])
	if err != nil {
		return ret
	}

	ret.Sign = sign
	c.queryCheckPointHashCache.Add(treePointHash, ret)

	return ret
}

func (c *CheckPointAPI) GetLastCheckPoint(ctx context.Context) *types.SignedCheckPoint {
	c.queryCheckPointCacheMu.Lock()
	defer c.queryCheckPointCacheMu.Unlock()

	treePoint := c.ftl.BlockChain().GetCheckPoint()
	log.Info("get last check point from local", "treePoint", treePoint)

	if p, ok := c.queryCheckPointCache.Get(treePoint.Hash()); ok {
		return p.(*types.SignedCheckPoint)
	}

	ret := &types.SignedCheckPoint{
		CheckPoint: treePoint,
	}

	key := c.checkPointPriKey
	if key == nil || reflect.ValueOf(key).IsNil() {
		return ret
	}

	hash := treePoint.Hash()
	sign, err := key.Sign(hash[:])
	if err != nil {
		return ret
	}

	ret.Sign = sign
	c.queryCheckPointCache.Add(hash, ret)

	return ret
}

func (c *CheckPointAPI) StartCreateCheckPoint(ctx context.Context) {
	c.ftl.BlockChain().StartCreateCheckPoint()
}
