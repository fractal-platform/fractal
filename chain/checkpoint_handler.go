// Copyright 2019 The go-fractal Authors
// This file is part of the go-fractal library.

// checkpoint_handler.go is main entry for hash tree

package chain

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/hashicorp/golang-lru"
)

const (
	CheckPointHeightSpacing uint64 = 100 // 8192
	CheckPointCreateHeight  uint64 = 30  // 1000
	CheckPointCreateDelay   uint64 = 20  // 50
	CheckPointVerifyDelay   uint64 = 40  // 100
)

var (
	errCheckPointNotFound   = errors.New("check point not found")
	errRemoteMsgNotSigned   = errors.New("remote message not signed")
	errRemoteMsgSignedError = errors.New("remote message signed error")
)

func (bc *BlockChain) StartCreateCheckPoint() {
	if bc.chainConfig.CheckPointEnable {
		bc.checkPointHandler.startCreateCheckPoint()
	}
}

func (bc *BlockChain) StopCreateCheckPoint() {
	bc.checkPointHandler.stopCreateCheckPoint()
}

func (bc *BlockChain) GetCheckPoint() *types.CheckPoint {
	if !bc.chainConfig.CheckPointEnable {
		genesisCheckPoint, _ := bc.GetLatestCheckPointBelowHeight(0, true)
		return genesisCheckPoint
	}
	return bc.checkPointHandler.getLocalLastCheckPoint()
}

func (bc *BlockChain) GetCheckPointByIndex(index uint64) (*types.CheckPoint, error) {
	return bc.checkPointHandler.getCheckPointByIndex(index)
}

func (bc *BlockChain) GetCheckPointByHash(hash common.Hash) *types.CheckPoint {
	if !bc.chainConfig.CheckPointEnable {
		genesisCheckPoint, _ := bc.GetLatestCheckPointBelowHeight(0, true)
		return genesisCheckPoint
	}
	latestCheckPoint := bc.checkPointHandler.getLocalLastCheckPoint()
	if latestCheckPoint == nil {
		return nil
	}
	if latestCheckPoint.FullHash == hash {
		return latestCheckPoint
	}
	for i := latestCheckPoint.Height / CheckPointHeightSpacing; ; i-- {
		checkPoint, err := bc.checkPointHandler.getCheckPointByIndex(i)
		if err != nil {
			bc.logger.Error("get checkpoint failed", "err", err)
			return nil
		}
		if checkPoint.FullHash == hash {
			return checkPoint
		}
		if i == 0 {
			return nil
		}
	}
}

// force means querying continuously until a checkpoint or genesis is queried
func (bc *BlockChain) GetLatestCheckPointBelowHeight(height uint64, force bool) (*types.CheckPoint, error) {
	if !bc.chainConfig.CheckPointEnable {
		return bc.checkPointHandler.getCheckPointByIndex(0) // genesis
	}

	greedy := uint64(bc.chainConfig.Greedy)
	var preCheckPointIndex uint64
	if height < CheckPointHeightSpacing-greedy-1 {
		preCheckPointIndex = 0
	} else if height >= (height/CheckPointHeightSpacing+1)*CheckPointHeightSpacing-greedy-1 {
		preCheckPointIndex = height/CheckPointHeightSpacing + 1
	} else {
		preCheckPointIndex = height / CheckPointHeightSpacing
	}

	last, err := bc.checkPointHandler.getCheckPointByIndex(preCheckPointIndex)
	if err == nil {
		return last, nil
	}

	if !force {
		return nil, err
	}

	// force
	if preCheckPointIndex >= 1 {
		for i := preCheckPointIndex - 1; ; i-- {
			if last, err := bc.checkPointHandler.getCheckPointByIndex(i); err == nil {
				return last, nil
			}

			if i == 0 {
				break // uint64 'i == 0' should not do 'i--'
			}
		}
	}

	return nil, errCheckPointNotFound // should not happen
}

// verify block if it is after check point rule
func (bc *BlockChain) meetCheckPointRule(block *types.Block) bool {
	if !bc.chainConfig.CheckPointEnable {
		return true
	}

	treePoint := bc.checkPointHandler.getLocalLastCheckPoint()
	currentBlock := bc.CurrentBlock()

	commonPrefix, found := bc.findCommonPrefixWithHeightLimit(currentBlock, block, treePoint.Height)
	if !found {
		return false
	}

	if treePoint.Height == commonPrefix.Header.Height {
		return treePoint.FullHash == commonPrefix.FullHash()
	} else {
		// commonPrefix.Header.Height > treePoint.Height
		return true
	}
}

type nodeBehavior interface {
	genCheckPointDistance() uint64
	verifyCheckPoint(checkPoint *types.CheckPoint) bool
	initLastCheckPoint(c *checkPointHandler)
	startCheck(c *checkPointHandler)
}

type checkPointHandler struct {
	blockChain   *BlockChain
	nodeBehavior nodeBehavior

	lastCheckPointCache atomic.Value
	queryCache          *lru.Cache // index -> checkPoint lru map
	queryCacheMu        sync.Mutex

	ctx    context.Context
	cancel context.CancelFunc

	running int32
}

func newCheckPointHandler(blockChain *BlockChain, checkPointNodeType types.CheckPointNodeTypeEnum) (*checkPointHandler, error) {
	var nodeBehavior nodeBehavior
	switch checkPointNodeType {
	case types.NormalNode:
		nodeBehavior = &normalNode{
			rpcAddress: types.CheckPointNodeRPC,
		}
	case types.SpecialNode:
		nodeBehavior = &specialNode{}
	}

	c := &checkPointHandler{
		blockChain:   blockChain,
		nodeBehavior: nodeBehavior,
	}

	var err error
	c.queryCache, err = lru.New(10)
	if err != nil {
		return nil, err
	}

	c.initLastCheckPoint()
	atomic.StoreInt32(&c.running, 0)

	return c, nil
}

func (c *checkPointHandler) startCreateCheckPoint() {
	if atomic.LoadInt32(&c.running) == 1 {
		return
	}

	log.Info("Start checking the previous check point")
	c.nodeBehavior.startCheck(c)

	log.Info("Start creating check point")
	events := make(chan types.ChainUpdateEvent, 10)
	sub := c.blockChain.SubscribeChainUpdateEvent(events)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	go c.createCheckPointLoop(events, sub)
	atomic.StoreInt32(&c.running, 1)
}

func (c *checkPointHandler) stopCreateCheckPoint() {
	if atomic.LoadInt32(&c.running) == 0 {
		return
	}
	atomic.StoreInt32(&c.running, 0)
	if c.cancel != nil {
		log.Info("Stop creating check point")
		c.cancel()
	}
}

func (c *checkPointHandler) createCheckPointLoop(events chan types.ChainUpdateEvent, sub event.Subscription) {
	defer sub.Unsubscribe()

	greedy := uint64(c.blockChain.chainConfig.Greedy)

Loop:
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				log.Error("checkPointHandler createCheckPointLoop: channel error")
				return
			}

			block := ev.Block

			if block.Header.Height <= CheckPointHeightSpacing || block.Header.Height%CheckPointHeightSpacing != c.nodeBehavior.genCheckPointDistance()-greedy-1 {
				continue
			}

			// avoid repetition
			lastCheckPoint := c.getLocalLastCheckPoint()
			if lastCheckPoint.Height == block.Header.Height-c.nodeBehavior.genCheckPointDistance() {
				continue
			}

			upBlockHeight := block.Header.Height - c.nodeBehavior.genCheckPointDistance() + CheckPointCreateHeight
			upBlock, err := c.blockChain.GetMainBranchBlock(upBlockHeight)
			if err != nil {
				log.Error("checkPointHandler createCheckPointLoop: get upBlock error", "upBlockHeight", upBlockHeight, "currentHeight", block.Header.Height)
				continue
			}

			// create
			tempBlock := upBlock
			for i := uint64(0); i < CheckPointCreateHeight; i++ {
				parentBlock := c.blockChain.GetBlock(tempBlock.Header.ParentFullHash)
				if parentBlock == nil {
					log.Error("checkPointHandler createCheckPointLoop: get parentBlock error", "parentHash", tempBlock.Header.ParentFullHash, "currentHash", tempBlock.FullHash(), "currentHeight", tempBlock.Header.Height)
					continue Loop
				}
				tempBlock = parentBlock
			}
			_, treePoint, err := c.blockChain.CreateHashTree(tempBlock.FullHash(), upBlock.FullHash())
			if err != nil {
				log.Error("checkPointHandler createCheckPointLoop: create hashTree error", "belowHash", tempBlock.FullHash(), "upHash", block.FullHash(), "err", err)
				continue
			}

			checkPoint := &types.CheckPoint{TreePoint: treePoint}

			// verify
			if !c.nodeBehavior.verifyCheckPoint(checkPoint) {
				log.Error("checkPointHandler createCheckPointLoop: checkPoint verify failed")
				// TODO: to be more graceful
				errStr := `
An irreversible error occurred in your chain data.
Please clear all chain data and start over.
1. Execute the command to delete the chain data: rm -r <your data dir>/chaindata
2. Restart`
				panic(errStr)
			}

			// save
			checkPointIndex := block.Header.Height / CheckPointHeightSpacing
			c.saveCheckPoint(checkPointIndex, checkPoint)
		case <-c.ctx.Done():
			return
		}
	}
}

// index must >= 1
func (c *checkPointHandler) createCheckPoint(index uint64) *types.CheckPoint {
	belowBlockHeight := index*CheckPointHeightSpacing - uint64(c.blockChain.chainConfig.Greedy) - 1
	upBlockHeight := belowBlockHeight + CheckPointCreateHeight

	belowBlock, err := c.blockChain.GetMainBranchBlock(belowBlockHeight)
	if err != nil {
		return nil
	}

	upBlock, err := c.blockChain.GetMainBranchBlock(upBlockHeight)
	if err != nil {
		return nil
	}

	_, treePoint, err := c.blockChain.CreateHashTree(belowBlock.FullHash(), upBlock.FullHash())
	if err != nil {
		log.Error("checkPointHandler createCheckPoint: create hashTree error", "belowHash", belowBlock.FullHash(), "upHash", upBlock.FullHash(), "err", err)
		return nil
	}

	checkPoint := &types.CheckPoint{TreePoint: treePoint}
	return checkPoint
}

func (c *checkPointHandler) initLastCheckPoint() {
	c.nodeBehavior.initLastCheckPoint(c)
}

func (c *checkPointHandler) saveCheckPoint(index uint64, checkPoint *types.CheckPoint) {
	log.Info("checkPointHandler saveCheckPoint", "index", index, "checkPoint", checkPoint)
	c.queryCacheMu.Lock()
	c.queryCache.Add(index, checkPoint)
	c.queryCacheMu.Unlock()

	db := c.blockChain.db
	dbaccessor.WriteCheckPointByIndex(db, index, checkPoint)
	oldLast := c.getLocalLastCheckPoint()
	if oldLast == nil || oldLast.Height < checkPoint.Height {
		c.lastCheckPointCache.Store(checkPoint)
		dbaccessor.WriteLastCheckPoint(db, checkPoint)
	}
}

func (c *checkPointHandler) getLocalLastCheckPoint() *types.CheckPoint {
	value := c.lastCheckPointCache.Load()
	if value == nil {
		return nil
	}

	return value.(*types.CheckPoint)
}

func (c *checkPointHandler) getCheckPointByIndex(index uint64) (*types.CheckPoint, error) {
	c.queryCacheMu.Lock()
	defer c.queryCacheMu.Unlock()

	if p, ok := c.queryCache.Get(index); ok {
		return p.(*types.CheckPoint), nil
	}

	db := c.blockChain.db
	treePoint := dbaccessor.ReadCheckPointByIndex(db, index)
	if treePoint == nil {
		return nil, errCheckPointNotFound
	}

	c.queryCache.Add(index, treePoint)
	return treePoint, nil
}

type specialNode struct{}

func (n *specialNode) genCheckPointDistance() uint64 {
	return CheckPointCreateHeight + CheckPointCreateDelay
}

func (n *specialNode) verifyCheckPoint(checkPoint *types.CheckPoint) bool {
	return true
}

func (n *specialNode) initLastCheckPoint(c *checkPointHandler) {
	db := c.blockChain.db
	last := dbaccessor.ReadLastCheckPoint(db)
	if last == nil {
		genesisHash := dbaccessor.ReadGenesisBlockHash(db) // genesis should not be nil
		checkPoint := &types.CheckPoint{TreePoint: &types.TreePoint{Height: 0, FullHash: genesisHash, MainChainHashList: []common.Hash{genesisHash}, HashPairs: []types.HashPairFullAcc{{genesisHash, common.Hash{}}}}}
		c.saveCheckPoint(0, checkPoint)
	} else {
		c.lastCheckPointCache.Store(last)
	}
}

func (n *specialNode) startCheck(c *checkPointHandler) {
	currentBlock := c.blockChain.CurrentBlock()
	var index uint64 // The previous checkpoint index that should be calculated
	if currentBlock.Header.Height <= CheckPointHeightSpacing {
		return
	} else if currentBlock.Header.Height%CheckPointHeightSpacing >= n.genCheckPointDistance()-uint64(c.blockChain.chainConfig.Greedy)-1 {
		index = currentBlock.Header.Height / CheckPointHeightSpacing
	} else {
		index = currentBlock.Header.Height/CheckPointHeightSpacing - 1
	}

	if index >= 1 {
		_, err := c.getCheckPointByIndex(index)
		if err == nil {
			return
		}
		log.Warn("startCheck: cannot get check point by index", "index", index, "err", err)
		checkPoint := c.createCheckPoint(index)
		log.Info("startCheck: calc check point", "index", index, "checkPoint", checkPoint)
		if checkPoint != nil {
			c.saveCheckPoint(index, checkPoint)
		}
	}
}

type normalNode struct {
	rpcAddress string
}

func (n *normalNode) genCheckPointDistance() uint64 {
	return CheckPointCreateHeight + CheckPointVerifyDelay
}

func (n *normalNode) verifyCheckPoint(checkPoint *types.CheckPoint) bool {
	remoteCheckPointHash, err := n.getRemoteCheckPointHashFromRPC()
	if err != nil {
		log.Error("normalNode getRemoteCheckPointFromRPC: get remote checkPoint error", "rpcAddress", n.rpcAddress, "err", err)
		return false
	}

	if checkPoint.Hash() != remoteCheckPointHash {
		log.Error("normalNode verifyCheckPoint: local tree point hash is different with remote tree point hash", "local", checkPoint.Hash(), "remote", remoteCheckPointHash, "localCheckPoint", checkPoint)
		return false
	}
	return true
}

func (n *normalNode) initLastCheckPoint(c *checkPointHandler) {
	if !c.blockChain.chainConfig.CheckPointEnable {
		genesisHash := dbaccessor.ReadGenesisBlockHash(c.blockChain.db) // genesis should not be nil
		checkPoint := &types.CheckPoint{TreePoint: &types.TreePoint{Height: 0, FullHash: genesisHash, MainChainHashList: []common.Hash{genesisHash}, HashPairs: []types.HashPairFullAcc{{genesisHash, common.Hash{}}}}}
		c.saveCheckPoint(0, checkPoint)
		return
	}

	checkPoint, err := n.getRemoteCheckPointFromRPC()
	if err != nil {
		panic(err)
	}

	var index uint64
	if checkPoint.Height == 0 {
		index = 0
	} else {
		index = checkPoint.Height/CheckPointHeightSpacing + 1
	}

	c.saveCheckPoint(index, checkPoint)
}

func (n *normalNode) startCheck(c *checkPointHandler) {}

func (n *normalNode) getRemoteCheckPointHashFromRPC() (common.Hash, error) {
	var (
		tryTimes = 0
		client   *rpcclient.Client
		err      error
	)

	for {
		if tryTimes == 5 {
			return common.Hash{}, err
		}
		client, err = rpcclient.Dial(n.rpcAddress)
		if err != nil {
			tryTimes++
			log.Error("getRemoteCheckPointHashFromRPC: connect to rpc error", "rpc", n.rpcAddress, "tryTimes", tryTimes)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	var signedCheckPointHash *types.SignedCheckPointHash
	err = client.Call(&signedCheckPointHash, "ftl_getLastCheckPointHash")
	if err != nil {
		log.Error("get last check point hash error", "err", err)
		return common.Hash{}, err
	}

	if len(signedCheckPointHash.Sign) == 0 {
		return common.Hash{}, errRemoteMsgNotSigned
	}

	// verify sign here
	pubKey, err := crypto.Ecrecover(signedCheckPointHash.Hash[:], signedCheckPointHash.Sign)
	if err != nil {
		log.Error("remote pubkey recover failed", "rpc", n.rpcAddress, "err", err)
		return common.Hash{}, err
	}
	if hexutil.Encode(pubKey) != types.CheckPointNodePubKeyStr {
		return common.Hash{}, errRemoteMsgSignedError
	}

	return signedCheckPointHash.Hash, nil
}

func (n *normalNode) getRemoteCheckPointFromRPC() (*types.CheckPoint, error) {
	var (
		tryTimes = 0
		client   *rpcclient.Client
		err      error
	)

	for {
		if tryTimes == 5 {
			return nil, err
		}
		client, err = rpcclient.Dial(n.rpcAddress)
		if err != nil {
			tryTimes++
			log.Error("getRemoteCheckPointFromRPC: connect to rpc error", "rpc", n.rpcAddress, "tryTimes", tryTimes)
			time.Sleep(3 * time.Second)
		} else {
			break
		}
	}

	var signedCheckPoint *types.SignedCheckPoint
	err = client.Call(&signedCheckPoint, "ftl_getLastCheckPoint")
	if err != nil {
		log.Error("get last check point error", "err", err)
		return nil, err
	}

	if len(signedCheckPoint.Sign) == 0 {
		return nil, errRemoteMsgNotSigned
	}

	// verify sign here
	checkPointHash := signedCheckPoint.CheckPoint.Hash()
	pubKey, err := crypto.Ecrecover(checkPointHash[:], signedCheckPoint.Sign)
	if err != nil {
		log.Error("remote pubkey recover failed", "rpc", n.rpcAddress, "err", err)
		return nil, err
	}
	if hexutil.Encode(pubKey) != types.CheckPointNodePubKeyStr {
		return nil, errRemoteMsgSignedError
	}

	return signedCheckPoint.CheckPoint, nil
}
