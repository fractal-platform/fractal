// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package network contains the implementation of network protocol handler for fractal.
package network

import (
	"errors"
	"fmt"
	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/protocol"
	Miner "github.com/fractal-platform/fractal/miner"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/p2p/discover"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/ratelimit"
	"sync"
	"time"
)

const (
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned Blocks, headers or node data.

	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096

	// txPackageSize
	txPackageChanSize = 32

	rate       = 1 << 20
	capability = 1 << 21
	waitTime   = time.Duration(90 * time.Second)

	dependErrCount = 6
)

// errIncompatibleConfig is returned if the requested protocols and configs are
// not compatible (low protocol version restrictions and high requirements).
var (
	errIncompatibleConfig = errors.New("incompatible configuration")
)

func errResp(code protocol.ErrCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

type blockChain interface {
	HasTxPackage(hash common.Hash) bool
	GetTxPackage(hash common.Hash) *types.TxPackage
	IsTxPackageInFuture(hash common.Hash) bool
	GetRelatedBlockForFutureTxPackage(hash common.Hash) common.Hash
	VerifyTxPackage(pkg *types.TxPackage) error
	FutureBlockTxPackages(blockHash common.Hash) types.TxPackages
	RemoveFutureBlockTxPackage(pkgHash common.Hash)

	HasBlock(hash common.Hash) bool
	CurrentBlock() *types.Block
	SendBlockExecutedFeed(block *types.Block)
	SetCurrentBlock(currentBlock *types.Block)
	FutureBlocks(relatedHash common.Hash) types.Blocks
	RemoveFutureBlocks(relatedHash common.Hash)
	FutureTxPackageBlocks(relatedTxPackageHash common.Hash) types.Blocks
	RemoveFutureTxPackageBlocks(relatedTxPackageHash common.Hash)
	Genesis() *types.Block
	GetBlock(hash common.Hash) *types.Block
	GetBlocksFromBlock(hash common.Hash, depth uint64, reverse bool) types.Blocks
	GetBlocksFromRoundRange(r1 uint64, r2 uint64) types.Blocks
	GetBlockBeforeCacheHeight(block *types.Block, cacheHeight uint8) (*types.Block, bool)

	VerifyBlock(block *types.Block, checkGreedy bool) (types.Blocks, common.Hash, common.Hash, error)
	InsertBlock(block *types.Block)
	InsertBlockNoCheck(block *types.Block)
	InsertPastBlock(block *types.Block) error

	// TrieNode retrieves a blob of data associated with a trie node (or code Hash)
	// either from ephemeral in-memory cache, or from persistent storage.
	TrieNode(hash common.Hash) ([]byte, error)
	GetChainConfig() *config.ChainConfig
	Database() dbwrapper.Database

	GetCheckPoints() *config.CheckPoints
}

type synchronizer interface {
	Start()
	Stop()
	IsSyncStatusNormal() bool
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer)
	ProcessNodeData(peer *Peer, data [][]byte)
	ProcessTxPackagesReq(peer *Peer, stage protocol.SyncStage, pkgHashes []common.Hash, bucket *ratelimit.Bucket, waitTime time.Duration)
	ProcessTxPackagesRsp(peer *Peer, stage protocol.SyncStage, pkgs []*types.TxPackage)
	ProcessBlocksReq(peer *Peer, stage protocol.SyncStage, roundFrom uint64, roundTo uint64) error
	ProcessBlocksRsp(peer *Peer, stage protocol.SyncStage, blocks types.Blocks)
	ProcessSyncPreBlocksForStateReq(peer *Peer, hash common.Hash) error
	ProcessSyncPreBlocksForStateRsp(blocks types.Blocks)
	ProcessSyncPostBlocksForStateReq(p *Peer, hashReq protocol.IntervalHashReq) error
	ProcessSyncPostBlocksForStateRsp(blocks types.Blocks)
	HandleHashesResponse(p *Peer, hashesRes protocol.SyncHashListRsp)
	HandleHashesRequest(p *Peer, hashesReq protocol.SyncHashListReq)
	DoPeerSync(p *Peer)
}

type ProtocolManager struct {
	networkID uint64

	// peers
	peers    *Peers
	maxPeers int

	// protocols
	SubProtocols []p2p.Protocol

	// interfaces
	blockchain blockChain

	// for new packed TxPackage broadcast
	packer       packer.Packer
	newPackedCh  chan packer.NewPackageEvent
	newPackedSub event.Subscription

	// for tx package fetcher
	txpkgFetcher *txpkgFetcher

	// TODO: should be removed
	mtx sync.Mutex

	// for sync
	syncQuitCh   chan struct{}
	synchronizer synchronizer

	// for tx
	txpool pool.Pool
	txsCh  chan pool.NewElemEvent
	txsSub event.Subscription

	// fot tx package
	txPkgPool pool.Pool
	txPkgCh   chan pool.NewElemEvent
	txPkgSub  event.Subscription

	// for new mined block broadcast
	miner            Miner.Miner
	newMinedBlockCh  chan types.NewMinedBlockEvent
	newMinedBlockSub event.Subscription

	// for block process
	blockProcessing  mapset.Set
	blockProcessLock sync.Mutex
	BlockProcessCh   chan *BlockWithVerifyFlag

	// for depend process
	dependMutex  sync.Mutex
	dependRecord map[string]*dependRecord

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup

	bucket *ratelimit.Bucket
}

// for depend error
type dependRecord struct {
	started          bool
	bestBlock        protocol.HashElem
	peerDependLength map[common.Hash]int
}

func (pm *ProtocolManager) GetPacker() packer.Packer {
	return pm.packer
}

// NewProtocolManager returns a new Fractal sub protocol manager. The Fractal sub protocol manages peers capable
// with the Fractal network.
func NewProtocolManager(networkID uint64, miner Miner.Miner, blockchain blockChain, packer packer.Packer, txpool pool.Pool, txPkgPool pool.Pool) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		networkID:       networkID,
		peers:           NewPeers(),
		blockchain:      blockchain,
		packer:          packer,
		syncQuitCh:      make(chan struct{}),
		miner:           miner,
		txpool:          txpool,
		txPkgPool:       txPkgPool,
		blockProcessing: mapset.NewSet(),
		BlockProcessCh:  make(chan *BlockWithVerifyFlag, 128),
		dependRecord:    make(map[string]*dependRecord),
	}

	manager.txpkgFetcher = newTxpkgFetcher(manager.peers)
	manager.bucket = ratelimit.NewBucketWithRate(rate, capability)

	// Initiate a sub-protocol for every implemented version we can handle
	manager.SubProtocols = make([]p2p.Protocol, 0, len(protocol.ProtocolVersions))
	for i, version := range protocol.ProtocolVersions {
		version := version // Closure for the run
		manager.SubProtocols = append(manager.SubProtocols, p2p.Protocol{
			Name:    protocol.ProtocolName,
			Version: version,
			Length:  protocol.ProtocolLengths[i],
			Run: func(p *p2p.Peer, rw p2p.MsgReadWriter) error {
				peer := manager.newPeer(int(version), p, rw)
				manager.wg.Add(1)

				defer manager.wg.Done()
				return manager.handle(peer)
			},
			NodeInfo: func() interface{} {
				return manager.NodeInfo()
			},
			PeerInfo: func(id discover.NodeID) interface{} {
				if p := manager.peers.Peer(fmt.Sprintf("%x", id[:8])); p != nil {
					return p.Info()
				}
				return nil
			},
		})
	}
	if len(manager.SubProtocols) == 0 {
		return nil, errIncompatibleConfig
	}

	return manager, nil
}

func (pm *ProtocolManager) SetSynchronizer(synchronizer synchronizer) {
	pm.synchronizer = synchronizer
}

func (pm *ProtocolManager) RemovePeer(id string, addBlack bool) {
	// Short circuit if the peer was already removed
	peer := pm.peers.Peer(id)
	if peer == nil {
		return
	}
	log.Info("Removing Fractal peer", "type", "console", "peer", id, "addBlack", addBlack)

	if err := pm.peers.Unregister(id); err != nil {
		log.Error("Peer removal failed", "peer", id, "err", err)
	}
	// Hard disconnect at the networking layer
	if peer != nil {
		if !addBlack {
			peer.Disconnect(p2p.DiscUselessPeer)
		} else {
			peer.Disconnect(p2p.DiscMaliciousPeer)
		}
	}
	// Unregister the peer from the synchronizer
	pm.synchronizer.RemovePeer(peer)
}

func (pm *ProtocolManager) Start(maxPeers int) {
	log.Info("Starting ProtocolManager")

	pm.maxPeers = maxPeers

	// broadcast transactions
	pm.txsCh = make(chan pool.NewElemEvent, txChanSize)
	pm.txsSub = pm.txpool.SubscribeNewElemEvent(pm.txsCh)
	go pm.txBroadcastLoop()

	// process txPackage
	pm.txPkgCh = make(chan pool.NewElemEvent, txPackageChanSize)
	pm.txPkgSub = pm.txPkgPool.SubscribeNewElemEvent(pm.txPkgCh)
	go pm.txPackageListenLoop()

	// broadcast mined Blocks
	if pm.miner != nil {
		pm.newMinedBlockCh = make(chan types.NewMinedBlockEvent, 10)
		pm.newMinedBlockSub = pm.miner.SubscribeNewMinedBlockEvent(pm.newMinedBlockCh)
		go pm.minedBroadcastLoop()
	}

	// broadcast packed packages
	pm.newPackedCh = make(chan packer.NewPackageEvent, 10)
	pm.newPackedSub = pm.packer.Subscribe(pm.newPackedCh)
	go pm.packageBroadcastLoop()

	// process blocks from network
	for i := 0; i < 8; i++ {
		go pm.blockProcessLoop(i)
	}

	// start sync
	go pm.synchronizer.Start()
}

func (pm *ProtocolManager) Stop() {
	log.Info("Stopping Fractal protocol")

	if pm.txsSub != nil {
		pm.txsSub.Unsubscribe() // quits txBroadcastLoop
	}

	if pm.txPkgSub != nil {
		pm.txPkgSub.Unsubscribe()
	}

	if pm.newMinedBlockSub != nil {
		pm.newMinedBlockSub.Unsubscribe() // quits blockBroadcastLoop
	}

	if pm.newPackedSub != nil {
		pm.newPackedSub.Unsubscribe() // quits packageBroadcastLoop
	}

	close(pm.BlockProcessCh)

	// Quit the sync loop.
	close(pm.syncQuitCh)
	pm.synchronizer.Stop()

	// Disconnect existing sessions.
	// This also closes the gate for any new registrations on the peer set.
	// sessions which are already established but not added to pm.peers yet
	// will exit when they try to register.
	pm.peers.Close()

	// wait for all peer handler goroutines and the loops to come down.
	pm.wg.Wait()

	log.Info("Fractal protocol stopped")
}

func (pm *ProtocolManager) newPeer(pv int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return NewPeer(pv, p, rw)
}

// handle is the callback invoked to manage the life cycle of an eth  When
// this function terminates, the peer is disconnected.
func (pm *ProtocolManager) handle(p *Peer) error {
	// Ignore maxPeers if this is a trusted peer
	if pm.peers.Len() >= pm.maxPeers && !p.Peer.Info().Network.Trusted {
		log.Info("reject peer", "len", pm.peers.Len(), "maxPeers", pm.maxPeers, "trusted", p.Peer.Info().Network.Trusted)
		return p2p.DiscTooManyPeers
	}
	p.Log().Info("Fractal peer connected", "type", "console", "name", p.Name())

	// Execute the Fractal handshake
	var (
		genesis    = pm.blockchain.Genesis()
		head       = pm.blockchain.CurrentBlock()
		fullHash   = head.FullHash()
		simpleHash = head.SimpleHash()
		height     = head.Header.Height
		round      = head.Header.Round
	)
	if err := p.Handshake(pm.networkID, height, round, fullHash, simpleHash, genesis.FullHash()); err != nil {
		p.Log().Debug("Fractal handshake failed", "err", err)
		return err
	}
	// Register the peer locally
	if err := pm.peers.Register(p); err != nil {
		p.Log().Error("Fractal peer registration failed", "err", err)
		return err
	}
	defer pm.RemovePeer(p.GetID(), false)

	pm.synchronizer.AddPeer(p)

	// Propagate existing transactions. new transactions appearing
	// after this will be sent via broadcasts.
	//pm.syncTransactions(p)

	// main loop. handle incoming messages.
	log.Debug("start protocol manager handle loop")
	for {
		if err := pm.handleMsg(p); err != nil {
			p.Log().Error("Fractal message handling failed", "err", err)
			return err
		}
	}
}

// handleMsg is invoked whenever an inbound message is received from a remote
//  The remote connection is torn down upon returning any error.
func (pm *ProtocolManager) handleMsg(p *Peer) error {
	// Read the next message from the remote peer, and ensure it's fully consumed
	msg, err := p.GetRW().ReadMsg()
	if err != nil {
		return err
	}
	if msg.Size > protocol.ProtocolMaxMsgSize {
		return errResp(protocol.ErrMsgTooLarge, "%v > %v", msg.Size, protocol.ProtocolMaxMsgSize)
	}
	defer msg.Discard()
	log.Debug("recv msg", "code", msg.Code)

	// Handle the message depending on its contents
	switch {
	case msg.Code == protocol.StatusMsg:
		// Status messages should never arrive after the handshake
		return errResp(protocol.ErrExtraStatusMsg, "uncontrolled status message")

	case msg.Code == protocol.NewBlockMsg:
		// Retrieve and decode the propagated block
		log.Debug("NewBlock received", "sync", pm.synchronizer.IsSyncStatusNormal())
		var requests []protocol.NewBlockData
		if err := msg.Decode(&requests); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		if pm.synchronizer.IsSyncStatusNormal() {
			for _, request := range requests {
				request.Block.ReceivedAt = msg.ReceivedAt
				request.Block.ReceivedFrom = p
				request.Block.ReceivedPath = types.BlockMined
				request.Block.Header.HopCount++
				log.Info("send to block process channel(NewBlockMsg)", "hash", request.Block.FullHash())
				pm.BlockProcessCh <- &BlockWithVerifyFlag{request.Block, true}
			}
		} else {
			log.Debug("ignore NewBlockMsg")
		}

	case msg.Code == protocol.GetBlocksMsg:
		// Decode the complex block query
		var query protocol.GetBlocksData
		if err := msg.Decode(&query); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Debug("recv GetBlocksMsg", "originHash", query.OriginHash, "roundFrom", query.RoundFrom, "roundTo", query.RoundTo)

		var blocks types.Blocks
		if query.OriginHash != (common.Hash{}) {
			blocks = pm.blockchain.GetBlocksFromBlock(query.OriginHash, query.Depth, query.Reverse)
		} else {
			blocks = pm.blockchain.GetBlocksFromRoundRange(query.RoundFrom, query.RoundTo)
		}
		return p.SendBlocks(blocks)

	case msg.Code == protocol.BlocksMsg:
		// A batch of headers arrived to one of our previous requests
		var blocks types.Blocks
		if err := msg.Decode(&blocks); err != nil {
			return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}

		log.Debug("recv BlocksMsg", "len(Blocks)", len(blocks))
		if pm.synchronizer.IsSyncStatusNormal() {
			for _, block := range blocks {
				block.ReceivedAt = msg.ReceivedAt
				block.ReceivedFrom = p
				// Fast sync starts all execution after pulling all the blocks and packages at the beginning, so there is no dependency problem. So it won't be fast sync here.
				block.ReceivedPath = types.BlockMined
				log.Info("send to block process channel(BlocksMsg)", "hash", block.FullHash())
				pm.BlockProcessCh <- &BlockWithVerifyFlag{block, true}
			}
		}

	case msg.Code == protocol.TxMsg:
		// Transactions can be processed, parse all of them and deliver to the pool
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}

		elems := make([]pool.Element, len(txs))
		for i, tx := range txs {
			// Validate and mark the remote transaction
			if tx == nil {
				return errResp(protocol.ErrDecode, "transaction %d is nil", i)
			}
			p.MarkTransaction(tx.Hash())
			elems[i] = tx
		}
		pm.txpool.AddRemotes(elems)

	case msg.Code == protocol.TxPackageHashMsg:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		// Mark the Hash as present at the remote node
		p.MarkTxPackage(hash)

		if pm.synchronizer.IsSyncStatusNormal() {
			if !pm.blockchain.HasTxPackage(hash) && !pm.blockchain.IsTxPackageInFuture(hash) {
				pm.txpkgFetcher.insertTask(hash)
			}
		}

	case msg.Code == protocol.GetTxPackageMsg:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		p.Log().Info("Receive a request for tx package", "hash", hash)
		pkg := pm.blockchain.GetTxPackage(hash)
		if pkg != nil {
			p.AsyncSendTxPackage(pkg)
		}

	case msg.Code == protocol.TxPackageMsg:
		var pkg types.TxPackage
		pkg.ReceivedAt = msg.ReceivedAt
		pkg.ReceivedFrom = p
		if err := msg.Decode(&pkg); err != nil {
			return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}
		p.MarkTxPackage(pkg.Hash())

		if pm.synchronizer.IsSyncStatusNormal() {
			go func() {
				(&pkg).IncreaseHopCount()
				pm.insertTxPackage(&pkg, true, true)

				// tell fetcher
				pm.txpkgFetcher.finishTask(p)
			}()
		}

	case msg.Code == protocol.GetNodeDataMsg:
		// Decode the retrieval message
		log.Info("Receive GetNodeDateMsg")
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			log.Error("Bad message")
			return err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash  common.Hash
			bytes int
			data  [][]byte
		)
		for bytes < softResponseLimit && len(data) < downloader.MaxStateFetch {
			// Retrieve the Hash of the next state entry
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				log.Info("rlp: end of list")
				break
			} else if err != nil {
				return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested state entry, stopping if enough was found
			if entry, err := pm.blockchain.TrieNode(hash); err == nil {
				data = append(data, entry)
				bytes += len(entry)
			} else {
				log.Error("failed to fetch trie node", "hash", hash, "err", err)
				return err
			}
		}
		pm.bucket.WaitMaxDuration(int64(bytes), waitTime)
		return p.SendNodeData(data)

	case msg.Code == protocol.NodeDataMsg:
		log.Info("receive NodeDateMsg from", "peer", p.Name())

		// A batch of node state data arrived to one of our previous requests
		var data [][]byte
		if err := msg.Decode(&data); err != nil {
			return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}
		pm.synchronizer.ProcessNodeData(p, data)

	case msg.Code == protocol.SyncHashListReqMsg:
		var req protocol.SyncHashListReq
		if err := msg.Decode(&req); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		//
		log.Info("recv sync hash list request", "req", req, "genesis.hash", pm.blockchain.Genesis().FullHash())
		pm.synchronizer.HandleHashesRequest(p, req)

	case msg.Code == protocol.SyncHashListResMsg:

		var hashesRes protocol.SyncHashListRsp
		if err := msg.Decode(&hashesRes); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("recv sync hash list response", "peer", p.GetID())

		pm.synchronizer.HandleHashesResponse(p, hashesRes)

	case msg.Code == protocol.SyncPreBlocksForStateReqMsg:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}
		log.Info("receive SyncPreBlocksForStateReq", "peer", p.Name(), "hash", hash)
		return pm.synchronizer.ProcessSyncPreBlocksForStateReq(p, hash)

	case msg.Code == protocol.SyncPreBlocksForStateRspMsg:
		var rsp protocol.FetchBlockRsp
		if err := msg.Decode(&rsp); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		pm.mtx.Lock()
		log.Info("insert txPackages for preBlock")
		for _, pkg := range rsp.TxPackages {
			pkg.ReceivedAt = msg.ReceivedAt
			pkg.ReceivedFrom = p

			p.MarkTxPackage(pkg.Hash())
			pm.insertTxPackage(pkg, false, false)
		}
		pm.mtx.Unlock()

		for _, block := range rsp.Blocks {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockFastSync
		}

		if len(rsp.Blocks) > 0 {
			pm.synchronizer.ProcessSyncPreBlocksForStateRsp(rsp.Blocks)
		}

	case msg.Code == protocol.SyncPostBlocksForStateReqMsg:
		var hashReq protocol.IntervalHashReq

		if err := msg.Decode(&hashReq); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		return pm.synchronizer.ProcessSyncPostBlocksForStateReq(p, hashReq)

	case msg.Code == protocol.SyncPostBlocksForStateRspMsg:
		var rsp protocol.FetchBlockRsp
		if err := msg.Decode(&rsp); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("Receive sync Blocks for state req", "peer", p.Name(), "blockes", len(rsp.Blocks), "ToRound", rsp.RoundTo, "pkgs", len(rsp.TxPackages), "finnished", rsp.Finished)

		pm.mtx.Lock()
		for _, pkg := range rsp.TxPackages {
			pkg.ReceivedAt = msg.ReceivedAt
			pkg.ReceivedFrom = p

			p.MarkTxPackage(pkg.Hash())
			pm.insertTxPackage(pkg, false, true)
		}
		pm.mtx.Unlock()

		for _, block := range rsp.Blocks {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockFastSync
		}

		if len(rsp.Blocks) > 0 {
			pm.synchronizer.ProcessSyncPostBlocksForStateRsp(rsp.Blocks)
		}
		if rsp.Finished {
			//Tell synchronizer transfer finished
			pm.synchronizer.ProcessSyncPostBlocksForStateRsp(types.Blocks{})
		}

	case msg.Code == protocol.GetPkgsForBlockSyncMsg:
		var query protocol.SyncPkgsReq
		if err := msg.Decode(&query); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		pm.synchronizer.ProcessTxPackagesReq(p, query.Stage, query.PkgHashes, pm.bucket, waitTime)

	case msg.Code == protocol.PkgsForBlockSyncMsg:
		var rsp protocol.SyncPkgsRsp
		if err := msg.Decode(&rsp); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("Recv pkgs msg for sync", "peer", p.Name(), "length", len(rsp.Pkgs))

		pm.mtx.Lock()
		for _, pkg := range rsp.Pkgs {
			pkg.ReceivedFrom = p
			pkg.ReceivedAt = msg.ReceivedAt

			p.MarkTxPackage(pkg.Hash())
			pm.insertTxPackage(pkg, false, true)
		}
		pm.mtx.Unlock()
		pm.synchronizer.ProcessTxPackagesRsp(p, rsp.Stage, rsp.Pkgs)

	case msg.Code == protocol.GetBlocksForBlockSyncMsg:
		var query protocol.SyncBlocksReq

		if err := msg.Decode(&query); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		if err := pm.synchronizer.ProcessBlocksReq(p, query.Stage, query.RoundFrom, query.RoundTo); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

	case msg.Code == protocol.BlocksForBlockSyncMsg:
		var rsp protocol.SyncBlocksRsp
		if err := msg.Decode(&rsp); err != nil {
			return errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("receive blocks for block sync", "peer", p.Name(), "stage", rsp.Stage, "RoundFrom", rsp.RoundFrom, "RoundTo", rsp.RoundTo, "blocks", len(rsp.Blocks))

		for _, block := range rsp.Blocks {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockFastSync
			log.Info("BlocksForBlockSyncMsg block", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash())
		}
		if rsp.Blocks == nil {
			pm.synchronizer.ProcessBlocksRsp(p, rsp.Stage, types.Blocks{})
		} else {
			pm.synchronizer.ProcessBlocksRsp(p, rsp.Stage, rsp.Blocks)
		}

	default:
		return errResp(protocol.ErrInvalidMsgCode, "%v", msg.Code)
	}
	return nil
}

// BroadcastBlock will propagate a block to all neighbors
func (pm *ProtocolManager) BroadcastBlock(block *types.Block) {
	hash := block.FullHash()
	peers := pm.peers.PeersWithoutBlock(hash)

	// Send the block to all peers that does not know this block
	for _, peer := range peers {
		peer.AsyncSendNewBlock(block)
	}
	log.Info("Propagated block", "Hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))
}

func (pm *ProtocolManager) FinishDepend(p *Peer) {
	pm.dependMutex.Lock()
	defer pm.dependMutex.Unlock()

	record, ok := pm.dependRecord[p.id]
	if !ok {
		log.Error("finish depend err failed", "peerId", p.id)
		return
	}
	record.started = false
	record.peerDependLength = make(map[common.Hash]int)
	log.Info("finish depend err success")
}

func (pm *ProtocolManager) MarkDepend(p *Peer) {
	pm.dependMutex.Lock()
	defer pm.dependMutex.Unlock()

	log.Info("mark depend error", "peer", p)
	pm.dependRecord[p.id] = &dependRecord{
		started:          true,
		bestBlock:        protocol.HashElem{},
		peerDependLength: make(map[common.Hash]int),
	}
}

func (pm *ProtocolManager) markBestBlock(p *Peer, simpleHash common.Hash, height uint64, round uint64) {
	pm.dependMutex.Lock()
	defer pm.dependMutex.Unlock()

	if record, ok := pm.dependRecord[p.id]; !ok {
		pm.dependRecord[p.id] = &dependRecord{
			started:          false,
			bestBlock:        protocol.HashElem{Height: height, Hash: simpleHash, Round: round},
			peerDependLength: make(map[common.Hash]int),
		}
	} else {
		hashElem := record.bestBlock
		if headerCompare(hashElem.Hash, hashElem.Height, hashElem.Round, simpleHash, height, round) < 0 {
			record.bestBlock = protocol.HashElem{Height: height, Hash: simpleHash, Round: round}
		}
	}
}

func (pm *ProtocolManager) doDependErr(p *Peer, dependHash common.Hash, dependedHash common.Hash) {
	pm.dependMutex.Lock()
	defer pm.dependMutex.Unlock()

	peerId := p.id
	record, ok := pm.dependRecord[peerId]
	if !ok {
		pm.dependRecord[p.id] = &dependRecord{
			started:          false,
			bestBlock:        protocol.HashElem{},
			peerDependLength: make(map[common.Hash]int),
		}
		record = pm.dependRecord[p.id]
	}

	dependCount, dependOk := record.peerDependLength[dependHash]
	dependedCount, dependedOk := record.peerDependLength[dependedHash]
	log.Info("handle depend error", "peer", p, "dependHash", dependHash, "dependCount", dependCount, "dependedHash", dependedHash, "dependedCount", dependedCount)
	if dependedOk {
		if dependOk {
			record.peerDependLength[dependedHash] = utils.MaxOf(dependCount+1, dependedCount)
		}
	} else {
		if dependOk {
			record.peerDependLength[dependedHash] = dependCount + 1
		} else {
			record.peerDependLength[dependedHash] = 1
		}
	}

	if record.peerDependLength[dependedHash] < dependErrCount {
		return
	}

	if !record.started {
		record.started = true
		log.Error("depend error trigger peer sync", "peerId", peerId, "count", record.peerDependLength[dependedHash], "started", record.started)
		//do peer sync
		go pm.synchronizer.DoPeerSync(p)
		return
	}
	log.Info("depend error already trigger peer sync", "peerId", p.id, "count", record.peerDependLength[dependedHash], "started", record.started)
}

func (pm *ProtocolManager) insertBlockInternal(p *Peer, block *BlockWithVerifyFlag) bool {
	hash := block.Block.FullHash()
	peerID := "-"
	if p == nil && block.Block.ReceivedFrom != nil {
		p = block.Block.ReceivedFrom.(*Peer)
	}
	if p != nil {
		peerID = p.GetID()
	}
	pm.markBestBlock(p, block.Block.SimpleHash(), block.Block.Header.Height, block.Block.Header.Round)

	// Run the import on a new thread
	log.Info("Importing propagated block", "peer", peerID, "height", block.Block.Header.Height,
		"round", block.Block.Header.Round, "hash", hash, "simpleHash", block.Block.SimpleHash(),
		"parent", block.Block.Header.ParentFullHash, "duration", common.PrettyDuration(time.Since(block.Block.ReceivedAt)))

	// Quickly validate the header and propagate the block if it passes
	var err error
	var dependBlockHash, dependPkgHash common.Hash
	if block.Verify {
		switch _, dependBlockHash, dependPkgHash, err = pm.blockchain.VerifyBlock(block.Block, true); err {
		case nil:
			// All ok, quickly propagate to our peers
			go pm.BroadcastBlock(block.Block)

		case chain.ErrConfirmUnknownBlock, chain.ErrCannotFindParentBlock:
			// Something went wrong
			log.Error("Propagated block verification failed", "peer", peerID, "height", block.Block.Header.Height, "hash", hash, "err", err, "miss", dependBlockHash)

			// mark depend error
			pm.doDependErr(p, block.Block.FullHash(), dependBlockHash)
			if record, ok := pm.dependRecord[p.id]; !ok || (ok && !record.started) {
				p.RequestOneBlock(dependBlockHash)
			}
			return false

		case chain.ErrBlockTxPackageMissing:
			// Something went wrong
			log.Error("Propagated block verification failed", "peer", peerID, "Height", block.Block.Header.Height, "Hash", hash, "err", err, "miss", dependPkgHash)
			p.RequestTxPackage(dependPkgHash)
			return false
		case chain.ErrBlockNotMeetGreedy:
			log.Error("Propagated block verification failed", "peer", peerID, "Height", block.Block.Header.Height, "Hash", hash, "err", err)
			pm.RemovePeer(peerID, false)
			return false
		default:
			// Something went very wrong, drop the peer
			log.Error("Propagated block verification failed", "peer", peerID, "Height", block.Block.Header.Height, "Hash", hash, "err", err)
			pm.RemovePeer(peerID, false)
			return false
		}
	}
	pm.blockchain.InsertBlock(block.Block)
	return true
}

// BroadcastTxPackage will either propagate a tx package to it's peers, or
// will only announce it's availability (depending what's requested).
func (pm *ProtocolManager) BroadcastTxPackage(pkg *types.TxPackage, propagate bool) {
	hash := pkg.Hash()
	peers := pm.peers.PeersWithoutTxPackage(hash)

	// If propagation is requested, send to a subset of the peer
	if propagate {
		// Send the block to a subset of our peers
		for _, peer := range peers {
			peer.AsyncSendTxPackage(pkg)
		}
		log.Debug("Propagated tx package", "Hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(pkg.ReceivedAt)))
		return
	}
	// Otherwise if the block is indeed in out own chain, announce it
	if pm.blockchain.HasTxPackage(hash) {
		for _, peer := range peers {
			peer.AsyncSendTxPackageHash(pkg)
		}
		log.Debug("Announced tx package", "Hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(pkg.ReceivedAt)))
	}
}

// insert tx package
func (pm *ProtocolManager) insertTxPackage(pkg *types.TxPackage, broadcast bool, check bool) bool {
	peer := pkg.ReceivedFrom.(*Peer)
	hash := pkg.Hash()

	if pm.blockchain.HasTxPackage(hash) {
		return false
	}

	// Run the import on a new thread
	//log.Info("Importing propagated tx package", "peer", peer.GetID(), "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash)

	//
	if check {
		err := pm.blockchain.VerifyTxPackage(pkg)
		if err != nil {
			// Something went very wrong, drop the peer

			log.Error("verify Propagated tx package failed", "peer", peer.GetID(), "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash, "err", err)
			if err == chain.ErrTxPackageRelatedBlockNotFound {
				//pkg.ReceivedFrom.(*Peer).RequestOneBlock(pkg.BlockFullHash())
				return false
			}

			pm.RemovePeer(peer.GetID(), false)
			return false
		}
	}

	if broadcast {
		// If verify succeeded, broadcast the tx package
		go pm.BroadcastTxPackage(pkg, false)
	}

	//elapse := time.Duration(pkg.ReceivedAt.UnixNano()/1e6 - int64(pkg.GenTime()))
	//log.Info("insert Propagated tx package", "elapse", elapse, "hop", pkg.HopCount(), "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash, "peer", peer.GetID())

	// Run the actual import
	if err := pm.packer.InsertRemoteTxPackage(pkg); err != nil {
		log.Error("insert propagated tx package into pool failed", "peer", peer.GetID(), "packer", pkg.Packer(), "nonce", pkg.Nonce(), "hash", hash, "err", err)
	}

	return true
}

// Packer broadcast loop
func (pm *ProtocolManager) packageBroadcastLoop() {
	pm.wg.Add(1)
	defer pm.wg.Done()

	// automatically stops if unsubscribe
	for {
		select {
		case e := <-pm.newPackedCh:
			for _, pkg := range e.Pkgs {
				pm.BroadcastTxPackage(pkg, true)
			}
		case <-pm.newPackedSub.Err():
			return
		}
	}
}

// block process loop
func (pm *ProtocolManager) blockProcessLoop(index int) {
	pm.wg.Add(1)
	defer pm.wg.Done()

	for block := range pm.BlockProcessCh {
		log.Info("process block start", "index", index, "hash", block.Block.FullHash())
		pm.blockProcessLock.Lock()
		if !pm.blockProcessing.Contains(block.Block.FullHash()) && !pm.blockchain.HasBlock(block.Block.FullHash()) {
			pm.blockProcessing.Add(block.Block.FullHash())
			pm.blockProcessLock.Unlock()

			var p *Peer
			if block.Block.ReceivedFrom != nil {
				p = block.Block.ReceivedFrom.(*Peer)

				// Mark the peer as owning the block and schedule it for import
				p.MarkBlock(block.Block.FullHash())
			}

			ok := pm.insertBlockInternal(p, block)
			if ok {
				// Update the peers head if better than the previous
				var (
					trueFullHash   = block.Block.FullHash()
					trueSimpleHash = block.Block.SimpleHash()
					trueHeight     = block.Block.Header.Height
					trueRound      = block.Block.Header.Round
				)
				if p.CompareTo(trueSimpleHash, trueHeight, trueRound) < 0 {
					p.SetHead(trueFullHash, trueSimpleHash, trueHeight, trueRound)
				}

				// process future Blocks
				go func() {
					for _, futureBlock := range pm.blockchain.FutureBlocks(block.Block.FullHash()) {
						pm.BlockProcessCh <- &BlockWithVerifyFlag{futureBlock, true}
						log.Info("futureBlock", "Hash", futureBlock.FullHash(), "Height", futureBlock.Header.Height)
					}
				}()
				pm.blockchain.RemoveFutureBlocks(block.Block.FullHash())

				// process future tx packages
				for _, futureTxPackage := range pm.blockchain.FutureBlockTxPackages(block.Block.FullHash()) {
					log.Info("futureTxPackage", "pkgHash", futureTxPackage.Hash(), "blockHash", block.Block.FullHash())
					if pm.insertTxPackage(futureTxPackage, true, true) {
						pm.blockchain.RemoveFutureBlockTxPackage(futureTxPackage.Hash())
					}
				}
			}

			pm.blockProcessLock.Lock()
			pm.blockProcessing.Remove(block.Block.FullHash())
			pm.blockProcessLock.Unlock()
		} else {
			log.Debug("blockProcessLoop:have been in blockProcessing Set or in blockChain", "Hash", block.Block.FullHash(), "Height", block.Block.Header.Height)
			pm.blockProcessLock.Unlock()
		}
		log.Info("process block over", "index", index, "hash", block.Block.FullHash())
	}
}

// Mined broadcast loop
func (pm *ProtocolManager) minedBroadcastLoop() {
	pm.wg.Add(1)
	defer pm.wg.Done()

	// automatically stops if unsubscribe
	for {
		select {
		case e := <-pm.newMinedBlockCh:
			block := e.Block
			if block != nil {
				pm.BroadcastBlock(block)
			} else {
				return
			}
		case <-pm.newMinedBlockSub.Err():
			return
		}
	}
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (pm *ProtocolManager) NodeInfo() *NodeInfo {
	currentBlock := pm.blockchain.CurrentBlock()
	return &NodeInfo{
		Network: pm.networkID,
		Height:  currentBlock.Header.Height,
		Genesis: pm.blockchain.Genesis().FullHash(),
		Head:    currentBlock.FullHash(),
	}
}

type BlockWithVerifyFlag struct {
	Block  *types.Block
	Verify bool
}

// NodeInfo represents a short summary of the Fractal sub-protocol metadata
// known about the host
type NodeInfo struct {
	Network uint64      `json:"network"` // Fractal network ID (1=Mainnet, 2=Testnet)
	Height  uint64      `json:"Height"`  // Height of the host's blockchain
	Genesis common.Hash `json:"genesis"` // SHA3 Hash of the host's genesis block
	Head    common.Hash `json:"head"`    // SHA3 Hash of the host's best owned block
}
