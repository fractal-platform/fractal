// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package network contains the implementation of network protocol handler for fractal.
package network

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/ftl/protocol"
	Miner "github.com/fractal-platform/fractal/miner"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/p2p/discover"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/ratelimit"
)

const (
	softResponseLimit = 2 * 1024 * 1024 // Target maximum size of returned Blocks, headers or node data.

	// txChanSize is the size of channel listening to NewTxsEvent.
	// The number is referenced from the size of tx pool.
	txChanSize = 4096
)

// errIncompatibleConfig is returned if the requested protocols and configs are
// not compatible (low protocol version restrictions and high requirements).
var (
	errIncompatibleConfig = errors.New("incompatible configuration")
)

func errResp(code protocol.ErrCode, format string, v ...interface{}) error {
	return fmt.Errorf("%v - %v", code, fmt.Sprintf(format, v...))
}

type blockchain interface {
	HasTxPackage(hash common.Hash) bool
	IsTxPackageInFuture(hash common.Hash) bool
	GetTxPackage(hash common.Hash) *types.TxPackage
	VerifyTxPackage(pkg *types.TxPackage) error

	HasBlock(hash common.Hash) bool
	GetBlock(hash common.Hash) *types.Block
	CurrentBlock() *types.Block
	Genesis() *types.Block

	VerifyBlock(block *types.Block, checkGreedy bool) (types.Blocks, common.Hash, int, common.Hash, error)
	InsertBlock(block *types.Block)

	SubscribeFutureBlockEvent(ch chan<- types.FutureBlockEvent) event.Subscription
	SubscribeFutureTxPackageEvent(ch chan<- types.FutureTxPackageEvent) event.Subscription

	// TrieNode retrieves a blob of data associated with a trie node (or code Hash)
	// either from ephemeral in-memory cache, or from persistent storage.
	TrieNode(hash common.Hash) ([]byte, error)
}

type packer interface {
	InsertRemoteTxPackage(pkg *types.TxPackage) error

	Subscribe(ch chan<- types.TxPackages) event.Subscription
}

type synchronizer interface {
	Start()
	Stop()
	IsSyncStatusNormal() bool
	AddPeer(peer *Peer)
	RemovePeer(peer *Peer)
	ProcessNodeDataRsp(peer *Peer, data [][]byte)
	ProcessTxPackagesReq(peer *Peer, reqID uint64, stage protocol.SyncStage, pkgHashes []common.Hash)
	ProcessTxPackagesRsp(peer *Peer, reqID uint64, stage protocol.SyncStage, pkgs []*types.TxPackage)
	ProcessBlocksReq(peer *Peer, reqID uint64, stage protocol.SyncStage, hashReqs []common.Hash, roundFrom uint64, roundTo uint64) error
	ProcessBlocksRsp(peer *Peer, reqID uint64, stage protocol.SyncStage, blocks types.Blocks)
	HandleHashTreeRequest(p *Peer, hashTreeReq protocol.SyncHashTreeReq)
	HandleHashTreeResponse(p *Peer, hashTreeRes protocol.SyncHashTreeRsp)
	ProcessBestPeerBlocksReq(p *Peer, hashReq protocol.IntervalHashReq) error
	ProcessBestPeerBlocksRsp(p *Peer, blocks types.Blocks)
	HandleHashesResponse(p *Peer, hashesRes protocol.SyncHashListRsp)
	HandleHashesRequest(p *Peer, hashesReq protocol.SyncHashListReq)
	DoPeerSync(p *Peer)
	GetPeerSyncThreshold() int
}

type ProtocolManager struct {
	networkID uint64

	// peers
	peers    *Peers
	maxPeers int

	// protocols
	SubProtocols []p2p.Protocol

	//
	handlers []handler

	// interfaces
	chain blockchain

	// for tx package fetcher
	txpkgFetcher *txpkgFetcher

	// for block process
	blockProcessing  mapset.Set
	blockProcessLock sync.Mutex
	BlockProcessCh   chan *BlockWithVerifyFlag

	// for sync
	synchronizer synchronizer

	// for tx
	txPool pool.Pool
	txsCh  chan pool.NewElemEvent
	txsSub event.Subscription

	// fot tx package
	txPkgPool pool.Pool

	// for new mined block broadcast
	miner            Miner.Miner
	newMinedBlockCh  chan types.NewMinedBlockEvent
	newMinedBlockSub event.Subscription

	// for new packed TxPackage broadcast
	packer       packer
	newPackedCh  chan types.TxPackages
	newPackedSub event.Subscription

	// for future process
	futureBlockCh      chan types.FutureBlockEvent
	futureBlockSub     event.Subscription
	futureTxPackageCh  chan types.FutureTxPackageEvent
	futureTxPackageSub event.Subscription

	// for rate limit
	bucket *ratelimit.Bucket

	// wait group is used for graceful shutdowns during downloading
	// and processing
	wg sync.WaitGroup

	logger log.Logger
}

// NewProtocolManager returns a new Fractal sub protocol manager. The Fractal sub protocol manages peers capable
// with the Fractal network.
func NewProtocolManager(networkID uint64, miner Miner.Miner, blockchain blockchain, packer packer, txpool pool.Pool, txPkgPool pool.Pool) (*ProtocolManager, error) {
	// Create the protocol manager with the base fields
	manager := &ProtocolManager{
		networkID: networkID,
		peers:     NewPeers(),
		chain:     blockchain,
		miner:     miner,
		packer:    packer,
		txPool:    txpool,
		txPkgPool: txPkgPool,

		blockProcessing: mapset.NewSet(),
		BlockProcessCh:  make(chan *BlockWithVerifyFlag, 128),

		logger: log.NewSubLogger("m", "network"),
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

	pm.handlers = append(pm.handlers, newDefaultHandler(pm, pm.chain, pm.synchronizer, pm.packer, pm.txPool, pm.logger, pm.txpkgFetcher))
	pm.handlers = append(pm.handlers, newSyncHandler(pm, pm.chain, pm.synchronizer, pm.logger))

	// process blocks from network
	for i := 0; i < 8; i++ {
		go pm.blockProcessLoop(i)
	}

	// broadcast transactions
	pm.txsCh = make(chan pool.NewElemEvent, txChanSize)
	pm.txsSub = pm.txPool.SubscribeNewElemEvent(pm.txsCh)

	// broadcast mined Blocks
	pm.newMinedBlockCh = make(chan types.NewMinedBlockEvent, 10)
	pm.newMinedBlockSub = pm.miner.SubscribeNewMinedBlockEvent(pm.newMinedBlockCh)

	// broadcast packed packages
	pm.newPackedCh = make(chan types.TxPackages, 10)
	pm.newPackedSub = pm.packer.Subscribe(pm.newPackedCh)

	// future block
	pm.futureBlockCh = make(chan types.FutureBlockEvent, 10)
	pm.futureBlockSub = pm.chain.SubscribeFutureBlockEvent(pm.futureBlockCh)

	// future tx package
	pm.futureTxPackageCh = make(chan types.FutureTxPackageEvent, 10)
	pm.futureTxPackageSub = pm.chain.SubscribeFutureTxPackageEvent(pm.futureTxPackageCh)

	// loop
	go pm.loop()

	// start sync
	go pm.synchronizer.Start()
}

func (pm *ProtocolManager) Stop() {
	log.Info("Stopping Fractal protocol")

	if pm.txsSub != nil {
		pm.txsSub.Unsubscribe()
	}

	if pm.newMinedBlockSub != nil {
		pm.newMinedBlockSub.Unsubscribe()
	}

	if pm.newPackedSub != nil {
		pm.newPackedSub.Unsubscribe()
	}

	if pm.futureBlockSub != nil {
		pm.futureBlockSub.Unsubscribe()
	}

	if pm.futureTxPackageSub != nil {
		pm.futureTxPackageSub.Unsubscribe()
	}

	close(pm.BlockProcessCh)

	// Quit the sync loop.
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
	return NewPeer(pv, p, rw, pm.bucket)
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
		genesis    = pm.chain.Genesis()
		head       = pm.chain.CurrentBlock()
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

	// process message
	for _, h := range pm.handlers {
		ret, err := h.handleMsg(p, msg)
		if ret == HandleReturnDone {
			return err
		}
	}

	// if no handler process this message
	return errResp(protocol.ErrInvalidMsgCode, "%v", msg.Code)
}

// insert tx package
func (pm *ProtocolManager) insertTxPackage(pkg *types.TxPackage, broadcast bool, check bool) bool {
	peer := pkg.ReceivedFrom.(*Peer)
	hash := pkg.Hash()

	if pm.chain.HasTxPackage(hash) {
		return false
	}

	// Run the import on a new thread
	//log.Info("Importing propagated tx package", "peer", peer.GetID(), "packer", pkg.Packer(), "nonce", pkg.Nonce(), "Hash", hash)

	//
	if check {
		err := pm.chain.VerifyTxPackage(pkg)
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

// block process loop
func (pm *ProtocolManager) blockProcessLoop(index int) {
	pm.wg.Add(1)
	defer pm.wg.Done()

	for block := range pm.BlockProcessCh {
		log.Info("process block start", "index", index, "hash", block.Block.FullHash())
		pm.blockProcessLock.Lock()
		if !pm.blockProcessing.Contains(block.Block.FullHash()) && !pm.chain.HasBlock(block.Block.FullHash()) {
			pm.blockProcessing.Add(block.Block.FullHash())
			pm.blockProcessLock.Unlock()

			var p *Peer
			if block.Block.ReceivedFrom != nil {
				p = block.Block.ReceivedFrom.(*Peer)
			}

			pm.insertBlockInternal(p, block)

			pm.blockProcessLock.Lock()
			pm.blockProcessing.Remove(block.Block.FullHash())
			pm.blockProcessLock.Unlock()
		} else {
			log.Debug("blockProcessLoop:have been in blockProcessing Set or in blockchain", "Hash", block.Block.FullHash(), "Height", block.Block.Header.Height)
			pm.blockProcessLock.Unlock()
		}
		log.Info("process block over", "index", index, "hash", block.Block.FullHash())
	}
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

	// Run the import on a new thread
	log.Info("Importing propagated block", "peer", peerID, "height", block.Block.Header.Height,
		"round", block.Block.Header.Round, "hash", hash, "simpleHash", block.Block.SimpleHash(),
		"parent", block.Block.Header.ParentFullHash, "duration", common.PrettyDuration(time.Since(block.Block.ReceivedAt)))

	// Quickly validate the header and propagate the block if it passes
	var err error
	var dependBlockDepth int
	var dependBlockHash, dependPkgHash common.Hash
	if block.Verify {
		switch _, dependBlockHash, dependBlockDepth, dependPkgHash, err = pm.chain.VerifyBlock(block.Block, true); err {
		case nil:
			// All ok, quickly propagate to our peers
			go pm.BroadcastBlock(block.Block)

		case chain.ErrConfirmUnknownBlock, chain.ErrCannotFindParentBlock:
			// Something went wrong
			log.Error("Propagated block verification failed", "peer", peerID, "height", block.Block.Header.Height, "hash", hash, "err", err, "miss", dependBlockHash)
			p.RequestOneBlock(dependBlockHash)
			if dependBlockDepth >= pm.synchronizer.GetPeerSyncThreshold() {
				log.Info("Propagated block verification failed trigger peer sync", "peerSyncThreshold", pm.synchronizer.GetPeerSyncThreshold(), "dependBlockDepth", dependBlockDepth)
				pm.synchronizer.DoPeerSync(p)
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
	pm.chain.InsertBlock(block.Block)

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

	return true
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
	if pm.chain.HasTxPackage(hash) {
		for _, peer := range peers {
			peer.AsyncSendTxPackageHash(pkg)
		}
		log.Debug("Announced tx package", "Hash", hash, "recipients", len(peers), "duration", common.PrettyDuration(time.Since(pkg.ReceivedAt)))
	}
}

func (pm *ProtocolManager) loop() {
	pm.wg.Add(1)
	defer pm.wg.Done()

	const channelNumber = 5
	var closedChannelNumber int

	for {
		select {
		case pkgs := <-pm.newPackedCh:
			for _, pkg := range pkgs {
				pm.BroadcastTxPackage(pkg, true)
			}
		case event := <-pm.newMinedBlockCh:
			block := event.Block
			if block != nil {
				pm.BroadcastBlock(block)
			} else {
				return
			}
		case event := <-pm.txsCh:
			txs := append(types.Transactions{}, pool.ElemsToTxs(event.Ems)...)
			pm.BroadcastTxs(txs)
		case event := <-pm.futureBlockCh:
			block := event.Block
			pm.BlockProcessCh <- &BlockWithVerifyFlag{block, true}
		case event := <-pm.futureTxPackageCh:
			pkg := event.Pkg
			pm.insertTxPackage(pkg, false, true)

			// quit
		case <-pm.newPackedSub.Err():
			closedChannelNumber++
			if closedChannelNumber == channelNumber {
				return
			}
		case <-pm.newMinedBlockSub.Err():
			closedChannelNumber++
			if closedChannelNumber == channelNumber {
				return
			}
		case <-pm.txsSub.Err():
			closedChannelNumber++
			if closedChannelNumber == channelNumber {
				return
			}
		case <-pm.futureBlockSub.Err():
			closedChannelNumber++
			if closedChannelNumber == channelNumber {
				return
			}
		case <-pm.futureTxPackageSub.Err():
			closedChannelNumber++
			if closedChannelNumber == channelNumber {
				return
			}
		}
	}
}

// BroadcastTxs will propagate a batch of transactions to all peers which are not known to
// already have the given transaction.
func (pm *ProtocolManager) BroadcastTxs(txs types.Transactions) {
	var txset = make(map[*Peer]types.Transactions)

	// Broadcast transactions to a batch of peers not knowing about it
	for _, tx := range txs {
		peers := pm.peers.PeersWithoutTx(tx.Hash())
		for _, peer := range peers {
			txset[peer] = append(txset[peer], tx)
		}
		log.Debug("Broadcast transaction", "Hash", tx.Hash(), "recipients", len(peers))
	}
	// FIXME include this again: peers = peers[:int(math.Sqrt(float64(len(peers))))]
	for peer, txs := range txset {
		peer.AsyncSendTransactions(txs)
	}
}

// NodeInfo retrieves some protocol metadata about the running host node.
func (pm *ProtocolManager) NodeInfo() *NodeInfo {
	currentBlock := pm.chain.CurrentBlock()
	return &NodeInfo{
		Network: pm.networkID,
		Height:  currentBlock.Header.Height,
		Genesis: pm.chain.Genesis().FullHash(),
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
