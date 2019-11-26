// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package sync contains the implementation of fractal sync strategy.
package sync

import (
	"errors"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
)

const (
	// if the depth for depend-error is higher then [peerSyncThreshold], than PeerSync will start.
	peerSyncThreshold = 6

	// identifier for local node
	selfPeerId = "self"

	// if we got error in PeerSync with the peer, we won't PeerSync with him in [finishDependErrTime] seconds.
	finishDependErrTime = 600
)

var (
	errPeer                           = errors.New("peer process failed")
	errNotEnoughPeers                 = errors.New("not enough peers")
	errNoCommonPrefixInShortHashLists = errors.New("can't have common prefix in shortHashLists")
	errCanNotGetConsensus             = errors.New("failed to get to an agreement of hashLists")
	errFailedGetBlock                 = errors.New("failed get block")
)

// callbacks
type removePeerCallback func(id string, addBlack bool)

type Synchronizer struct {
	config     *config.SyncConfig
	syncQuitCh chan struct{}  // for quit
	syncQuitWg sync.WaitGroup // for shutdown sync
	log        log.Logger

	chain              blockchain
	miner              miner
	packer             packer.Packer
	removePeerCallback removePeerCallback
	blockProcessCh     chan *network.BlockWithVerifyFlag

	// status & mode
	status         atomic.Value
	fastSyncMode   atomic.Value
	fastSyncStatus atomic.Value

	// for peers management
	peers     map[string]peer // all p2p peers
	peersLock sync.RWMutex    //
	newPeerCh chan peer       // channel for new peer event

	lastHeadBlock *types.Block // if any wrong happens, reset currentBlock

	// for cp2fp
	cp2fp                  *CP2FPSync
	syncHashListChForCP2FP chan PeerHashElemList

	// for fast sync
	stateSync      *downloader.StateSync
	blockSync      *downloader.BlockFetcherByHash
	blockSyncRound *downloader.BlockFetcherByRound

	syncHashListChForFastSync chan PeerHashElemList
	hashTreeRevCh             chan protocol.SyncHashTreeRsp
	blocksForPreStateRevCh    chan []*types.Block // pre Blocks
	blocksForPostStateRevCh   chan []*types.Block // post Blocks
	fastSyncFinishedCh        chan struct{}
	fastSyncErrCh             chan struct{} //error when fastSync
	fastSyncQuitCh            chan struct{}

	// for peer sync
	syncHashListChForPeerSync chan PeerHashElemList
	fixPoint                  protocol.HashElem
	peerSyncStarted           map[string]bool
	peerSyncFinishedCh        chan peer
	peerSyncErrCh             chan peer
}

func NewSynchronizer(chain blockchain, miner miner, packer packer.Packer, removePeerCallback removePeerCallback, blockProcessCh chan *network.BlockWithVerifyFlag, conf *config.SyncConfig) *Synchronizer {
	sync := &Synchronizer{
		config:     conf,
		syncQuitCh: make(chan struct{}),
		log:        log.NewSubLogger("m", "sync"),

		chain:              chain,
		miner:              miner,
		packer:             packer,
		removePeerCallback: removePeerCallback,
		blockProcessCh:     blockProcessCh,

		peers:     make(map[string]peer),
		newPeerCh: make(chan peer, 20),

		syncHashListChForCP2FP: make(chan PeerHashElemList, 16),

		hashTreeRevCh:             make(chan protocol.SyncHashTreeRsp),
		blocksForPreStateRevCh:    make(chan []*types.Block),
		blocksForPostStateRevCh:   make(chan []*types.Block),
		fastSyncFinishedCh:        make(chan struct{}),
		fastSyncErrCh:             make(chan struct{}, 2),
		fastSyncQuitCh:            make(chan struct{}),
		syncHashListChForFastSync: make(chan PeerHashElemList, 16),

		syncHashListChForPeerSync: make(chan PeerHashElemList, 16),
		peerSyncStarted:           make(map[string]bool),
		peerSyncFinishedCh:        make(chan peer),
		peerSyncErrCh:             make(chan peer),
	}
	sync.changeSyncStatus(SyncStatusInit)
	sync.changeFastSyncMode(FastSyncModeNone)
	sync.changeFastSyncStatus(FastSyncStatusNone)
	sync.cp2fp = newCP2FPSync(sync.hashTreeRevCh, sync)
	return sync
}

func (s *Synchronizer) Start() {
	go s.loop() // trigger
}

func (s *Synchronizer) Stop() {
	close(s.syncQuitCh)
	s.syncQuitWg.Wait()
}

func (s *Synchronizer) AddPeer(p *network.Peer) {
	s.peersLock.Lock()
	s.peers[p.GetID()] = p
	s.log.Info("recv new peer event", "peers.Len()", len(s.peers), "peer", p.GetID())
	s.peersLock.Unlock()

	s.newPeerCh <- p
}

//no need to unregister peers in downloader, they need to be unregistered where they are used
func (s *Synchronizer) RemovePeer(p *network.Peer) {
	s.peersLock.Lock()
	delete(s.peers, p.GetID())
	s.log.Info("recv del peer event", "peers.Len()", len(s.peers), "peer", p.GetID())
	s.peersLock.Unlock()
}

func (s *Synchronizer) loop() {
	s.syncQuitWg.Add(1)
	defer s.syncQuitWg.Done()

	// Wait for different events to fire synchronisation operations
	for {
		select {
		case p := <-s.newPeerCh:
			status := s.GetSyncStatus()
			s.log.Info("new peer status", "status", status)
			switch status {
			case SyncStatusInit:
				s.doInit()
			case SyncStatusFastSync:
				//do nothing
			case SyncStatusNormal:
				go s.doCheckPeer(p)
			case SyncStatusPeerSync:
				//do nothing
			}

			// add to peers for cp2fp
			s.cp2fp.registerPeer(p)

		case <-s.fastSyncFinishedCh:
			status := s.GetSyncStatus()
			s.log.Info("fast sync finished", "status", status)
			if status == SyncStatusFastSync {
				if s.miner != nil {
					s.miner.Start()
				}

				// move to StatusNormal
				s.changeSyncStatus(SyncStatusNormal)
				s.lastHeadBlock = nil
			}

		case <-s.fastSyncErrCh:
			status := s.GetSyncStatus()
			s.log.Error("fast sync failed", "status", status)
			if status == SyncStatusFastSync {
				s.changeSyncStatus(SyncStatusInit)
				s.changeFastSyncMode(FastSyncModeNone)
				s.changeFastSyncStatus(FastSyncStatusNone)

				//set currentBlock
				if s.lastHeadBlock != nil {
					s.chain.SetCurrentBlock(s.lastHeadBlock)
				}

				s.chain.StartCreateCheckPoint()
				// do init again
				s.doInit()
			}

		case p := <-s.peerSyncFinishedCh:
			status := s.GetSyncStatus()
			s.log.Info("peer sync finished", "peer", p.GetID(), "status", status)
			if status == SyncStatusPeerSync {
				if s.miner != nil {
					s.miner.Start()
				}

				// move to StatusNormal
				s.changeSyncStatus(SyncStatusNormal)
				s.lastHeadBlock = nil

				// reset flag and do callback
				s.peerSyncStarted[p.GetID()] = false

				// restart cp2fp
				go s.cp2fp.startTask(s.chain.CurrentBlock().Header.Height, s.chain.CurrentBlock().FullHash(), s.chain.CurrentBlock().AccHash, s.getPeers())
			}

		case p := <-s.peerSyncErrCh:
			status := s.GetSyncStatus()
			s.log.Error("peer sync failed", "peer", p.GetID(), "status", status)
			if status == SyncStatusPeerSync {
				if s.miner != nil {
					s.miner.Start()
				}

				s.changeSyncStatus(SyncStatusNormal)

				//set currentBlock
				if s.lastHeadBlock != nil {
					s.chain.SetCurrentBlock(s.lastHeadBlock)
				}

				// wait some time
				time.AfterFunc(time.Duration(finishDependErrTime)*time.Second, func() {
					s.peerSyncStarted[p.GetID()] = false
				})

				// restart cp2fp
				go s.cp2fp.startTask(s.chain.CurrentBlock().Header.Height, s.chain.CurrentBlock().FullHash(), s.chain.CurrentBlock().AccHash, s.getPeers())

			}

		case <-s.syncQuitCh:
			status := s.GetSyncStatus()
			s.log.Error("sync quit", "status", status)
			if status == SyncStatusFastSync || status == SyncStatusPeerSync {
				close(s.fastSyncQuitCh)
			}
			return
		}
	}
}

func (s *Synchronizer) doInit() {
	if len(s.peers) < s.config.MinRegularPeerCount {
		s.log.Info("not enough peers for sync", "peerCount", len(s.peers), "MinRegularPeerCount", s.config.MinRegularPeerCount)
		return
	}

	diff, _ := s.getHeightDiffFromRegularPeers()
	if diff < s.config.HeightDiff {
		//sync from break point or checkpoint
		// change to normal state
		s.changeSyncStatus(SyncStatusNormal)
		peers := s.getPeers()
		go s.cp2fp.startTask(s.chain.CurrentBlock().Header.Height, s.chain.CurrentBlock().FullHash(), s.chain.CurrentBlock().AccHash, peers)

		return
	}

	if len(s.peers) >= s.config.MinFastSyncPeerCount {
		// change to fast sync state
		s.changeSyncStatus(SyncStatusFastSync)
		go s.doFastSync()
	} else {
		s.log.Info("not enough peers for fast sync", "peerCount", len(s.peers), "minFastSyncPeerCount", s.config.MinFastSyncPeerCount)
	}
}

func (s *Synchronizer) doCheckPeer(p peer) {
	_, _, height, _ := p.Head()
	currentBlock := s.chain.CurrentBlock()
	currentHeight := currentBlock.Header.Height
	s.log.Info("start to check new peer", "peer", p.GetID(), "currentHeight", currentHeight, "peerHeight", height)
	if currentHeight+peerSyncThreshold < height {
		s.DoPeerSync(p.(*network.Peer))
	}
}

func (s *Synchronizer) findAndCheckMainChain(interHashesMap map[string]protocol.HashElems, peers []peer, comPreCount int, comPrefixIndexMap map[string]int) ([]peer, error) {
	s.log.Info("start to find and check main chain", "len(interHashesMap)", len(interHashesMap), "peers", peers, "comPrefixIndexMap", comPrefixIndexMap)
	leftPeers, _, _, _, blockSyncHashList, err := s.findMainChainPeers(interHashesMap, peers, comPrefixIndexMap, comPreCount)
	if err != nil {
		if err.Error() == errNotEnoughPeers.Error() {
			s.log.Info("meet bad peer, not enough good peers", "err", err)
			return nil, err
		}
		s.log.Error("this group can't get to a consensus, remove all peers", "err", err)
		for _, peer := range peers {
			s.removePeerCallback(peer.GetID(), false)
		}
		return nil, err
	}
	if len(leftPeers) == 0 {
		s.log.Info("self chain is the best")
		return leftPeers, nil
	}

	bestPeer := getBestPeerByHead(leftPeers)
	//check main chain
	check, errPeers, err := s.doSyncAndCheckFixPoint(leftPeers, bestPeer, blockSyncHashList, protocol.HashElem{}, true)
	if err != nil || !check || (len(peers)-len(errPeers) < comPreCount) {
		s.log.Error("do fast sync checkMainChain failed", "err", err, "check", check)
		for _, peer := range errPeers {
			s.removePeerCallback(peer.GetID(), false)
		}
		return nil, err
	}
	return leftPeers, nil
}

//
func (s *Synchronizer) getHeightDiffFromRegularPeers() (int32, []peer) {
	currentHeight := s.chain.CurrentBlock().Header.Height
	peers, _ := s.randomChoosePeers(s.peers, s.config.MinRegularPeerCount)
	var highest uint64 = 0
	for _, p := range peers {
		_, _, height, _ := p.Head()
		s.log.Info("compare peer height", "peerId", p.GetID(), "peerHeight", height, "currentHeight", currentHeight)
		if height > highest {
			highest = height
		}
	}

	if currentHeight >= highest {
		return -1, []peer{}
	}
	diff := highest - currentHeight
	s.log.Info("get height diff ok", "diff", diff, "highest", highest, "currentHeight", currentHeight)
	return int32(diff), peers
}

func (s *Synchronizer) getLatestCheckPoint() types.TreePoint {
	return *s.chain.GetCheckPoint().TreePoint
}

//choose length number from bucketSize ,example : choose [1,2] of [0,1,2]
func generateDiffRandomNumbers(bucketSize int, length int) []int {

	rand.Seed(time.Now().UnixNano())
	intMap := make(map[int]bool)
	var result []int
	for len(intMap) < length {
		intMap[rand.Intn(bucketSize)] = true
	}
	//s.log.Info("generateDiffRandomNumbers ", "bucketSize", bucketSize, "length", length)
	for k := range intMap {
		result = append(result, k)
	}
	//s.log.Info("generateDiffRandomNumbers ", "result", result)
	return result
}

func (s *Synchronizer) randomChoosePeers(peerMap map[string]peer, count int) ([]peer, map[string]peer) {
	var peers []peer
	var peerMapResult = make(map[string]peer)
	for _, peer := range peerMap {
		peers = append(peers, peer)
	}
	var randomPeers []peer
	numbers := generateDiffRandomNumbers(len(peers), count)
	for _, v := range numbers {
		randomPeers = append(randomPeers, peers[v])
		peerMapResult[ peers[v].GetID()] = peers[v]
	}
	return randomPeers, peerMap
}

func (s *Synchronizer) lengthForStatesSync() int {
	var maxHeightDistance = utils.MaxOf(int(params.StakeRegisterHeightDistance), int(params.ConfirmHeightDistance))
	return maxHeightDistance + 2*int(s.chain.GetChainConfig().Greedy) + chain.MaxPackageHeightDelay
}

func (s *Synchronizer) getPeers() []peer {
	var peers []peer
	s.peersLock.RLock()
	defer s.peersLock.RUnlock()
	for _, p := range s.peers {
		peers = append(peers, p)
	}
	return peers
}

func (s *Synchronizer) GetConfig() *config.SyncConfig {
	return s.config
}

func (s *Synchronizer) GetPeerSyncThreshold() int {
	return peerSyncThreshold
}

type peer interface {
	GetID() string
	Name() string
	Closed() bool
	Head() (fullHash common.Hash, simpleHash common.Hash, height uint64, round uint64)
	CompareTo(simpleHash common.Hash, height uint64, round uint64) int

	RequestSyncHashList(syncStage protocol.SyncStage, syncType protocol.SyncHashType, hashEFrom protocol.HashElem, hashETo protocol.HashElem) error
	SendSyncHashList(reqID uint64, syncStage protocol.SyncStage, hashType protocol.SyncHashType, hashList protocol.HashElems) error

	// for fast sync
	RequestNodeData(hashes []common.Hash) error
	RequestSyncHashTree(hashFrom common.Hash, hashTo common.Hash) error
	SendSyncHashTree(reqID uint64, tree types.HashTree, point types.TreePoint) error
	RequestSyncBestPeerBlocks(hashEFrom protocol.HashElem, hashETo protocol.HashElem) error

	// for block sync
	RequestSyncPkgs(stage protocol.SyncStage, hashes []common.Hash) error
	RequestSyncBlocks(stage protocol.SyncStage, reqsByHash []common.Hash, reqsFrom uint64, reqsTo uint64) error
}

type blockchain interface {
	HasTxPackage(hash common.Hash) bool
	GetTxPackage(hash common.Hash) *types.TxPackage
	IsTxPackageInFuture(hash common.Hash) bool
	VerifyTxPackage(pkg *types.TxPackage) error

	CurrentBlock() *types.Block
	SendBlockExecutedFeed(block *types.Block)
	GetBlocksFromRoundRange(r1 uint64, r2 uint64) types.Blocks
	SetCurrentBlock(currentBlock *types.Block)
	InsertBlock(block *types.Block)
	InsertPastBlock(block *types.Block) error
	InsertBlockNoCheck(block *types.Block)
	VerifyBlock(block *types.Block, checkGreedy bool) (types.Blocks, common.Hash, int, common.Hash, error)
	GetChainConfig() *config.ChainConfig
	HasBlock(hash common.Hash) bool
	GetBlock(hash common.Hash) *types.Block
	Database() dbwrapper.Database
	Genesis() *types.Block
	GetLatestCheckPointBelowHeight(height uint64, force bool) (*types.CheckPoint, error)
	GetCheckPointByHash(hash common.Hash) *types.CheckPoint
	GetCheckPoint() *types.CheckPoint
	StartCreateCheckPoint()
	StopCreateCheckPoint()
	GetBreakPoint(checkpoint *types.Block, headBlock *types.Block) (*types.Block, *types.Block, error)
	VerifyBlockDepend(block *types.Block) (common.Hash, error)
	CreateHashTree(belowBlockHash common.Hash, upBlockHash common.Hash) (*types.HashTree, *types.TreePoint, error)
	Filter(hashes common.Hashes) common.Hashes
}

type miner interface {
	Start()
	Stop()
}

type PeerHashElemList struct {
	HashType protocol.SyncHashType
	Peer     *network.Peer
	HashList []*protocol.HashElem
}
