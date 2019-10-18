// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package p2p_handler contains the implementation of p2p handler for fractal.
package network

import (
	"errors"
	"fmt"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/deckarep/golang-set"
	"sync"
	"time"
)

var (
	errClosed            = errors.New("peer set is closed")
	errNotRegistered     = errors.New("peer is not registered")
	errAlreadyRegistered = errors.New("peer is already registered")
)

const (
	maxKnownTxs        = 32768 // Maximum transactions hashes to keep in the known list (prevent DOS)
	maxKnownBlocks     = 1024  // Maximum block hashes to keep in the known list (prevent DOS)
	maxKnownTxPackages = 4096  // Maximum tx package hashes to keep in the known list (prevent DOS)

	handshakeTimeout = 5 * time.Second
)

// PeerInfo represents a short summary of the Fractal sub-protocol metadata known
// about a connected peer.
type PeerInfo struct {
	Version    int    `json:"version"`    // Fractal protocol version negotiated
	Height     uint64 `json:"Height"`     // Height of the peer's best owned block
	Round      uint64 `json:"Round"`      // Round of the peer's best owned block
	FullHash   string `json:"fullHash"`   // SHA3 Hash of the peer's best owned block
	SimpleHash string `json:"simpleHash"` // SHA3 Hash of the peer's best owned block
}

type Peer struct {
	id      string
	version int // Protocol version negotiated

	*p2p.Peer
	rw p2p.MsgReadWriter

	headFullHash   common.Hash
	headSimpleHash common.Hash
	headHeight     uint64
	headRound      uint64
	lock           sync.RWMutex

	pipe *taskPipe

	knownTxs        mapset.Set    // Set of transaction hashes known to be known by this peer
	knownTxPackages mapset.Set    // Set of tx package hashes known to be known by this peer
	knownBlocks     mapset.Set    // Set of block hashes known to be known by this peer
	term            chan struct{} // Termination channel to stop the broadcaster
	closed          bool
}

func NewPeer(version int, p *p2p.Peer, rw p2p.MsgReadWriter) *Peer {
	return &Peer{
		id:              fmt.Sprintf("%x", p.ID().Bytes()[:8]),
		version:         version,
		Peer:            p,
		rw:              rw,
		pipe:            newTaskPipe(),
		knownTxs:        mapset.NewSet(),
		knownBlocks:     mapset.NewSet(),
		knownTxPackages: mapset.NewSet(),
		term:            make(chan struct{}),
		closed:          false,
	}
}

// close signals the broadcast goroutine to terminate.
func (p *Peer) close() {
	close(p.term)
	p.closed = true
}

// GetID return the id of current peer
func (p *Peer) GetID() string {
	return p.id
}

func (p *Peer) GetRW() p2p.MsgReadWriter {
	return p.rw
}

// Info gathers and returns a collection of metadata known about a peer.
func (p *Peer) Info() *PeerInfo {
	fullHash, simpleHash, height, round := p.Head()
	return &PeerInfo{
		Version:    p.version,
		Height:     height,
		Round:      round,
		FullHash:   fullHash.Hex(),
		SimpleHash: simpleHash.Hex(),
	}
}

func (p *Peer) Closed() bool {
	return p.closed
}

// Head retrieves a copy of the current head block info of the peer.
func (p *Peer) Head() (fullHash common.Hash, simpleHash common.Hash, height uint64, round uint64) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	copy(fullHash[:], p.headFullHash[:])
	copy(simpleHash[:], p.headSimpleHash[:])
	return fullHash, simpleHash, p.headHeight, p.headRound
}

// SetHead updates the head block info.
func (p *Peer) SetHead(fullHash common.Hash, simpleHash common.Hash, height uint64, round uint64) {
	p.lock.Lock()
	defer p.lock.Unlock()

	copy(p.headFullHash[:], fullHash[:])
	copy(p.headSimpleHash[:], simpleHash[:])
	p.headHeight = height
	p.headRound = round
}

// if peer's head is higher, return 1
func (p *Peer) CompareTo(simpleHash common.Hash, height uint64, round uint64) int {
	return headerCompare(p.headSimpleHash, p.headHeight, p.headRound, simpleHash, height, round)
}

func headerCompare(headSimpleHash common.Hash, headHeight uint64, headRound uint64, simpleHash common.Hash, height uint64, round uint64) int {
	if headHeight > height {
		return 1
	} else if headHeight == height {
		if headRound < round {
			return 1
		} else if headRound == round {
			if headSimpleHash.Hex() < simpleHash.Hex() {
				return 1
			} else if headSimpleHash.Hex() == simpleHash.Hex() {
				return 0
			}
		}
	}
	return -1
}

// MarkBlock marks a block as known for the peer, ensuring that the block will
// never be propagated to this particular peer.
func (p *Peer) MarkBlock(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known block Hash
	for p.knownBlocks.Cardinality() >= maxKnownBlocks {
		p.knownBlocks.Pop()
	}
	p.knownBlocks.Add(hash)
}

// MarkTransaction marks a transaction as known for the peer, ensuring that it
// will never be propagated to this particular peer.
func (p *Peer) MarkTransaction(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known transaction Hash
	for p.knownTxs.Cardinality() >= maxKnownTxs {
		p.knownTxs.Pop()
	}
	p.knownTxs.Add(hash)
}

// MarkTxPackage marks a tx package as known for the peer, ensuring that the tx package will
// never be propagated to this particular peer.
func (p *Peer) MarkTxPackage(hash common.Hash) {
	// If we reached the memory allowance, drop a previously known block Hash
	for p.knownTxPackages.Cardinality() >= maxKnownTxPackages {
		p.knownTxPackages.Pop()
	}
	p.knownTxPackages.Add(hash)
}

// HasBlock tells whether current peer knows the block
func (p *Peer) HasBlock(hash common.Hash) bool {
	return p.knownBlocks.Contains(hash)
}

// HasTransaction tells whether current peer knows the transaction
func (p *Peer) HasTransaction(hash common.Hash) bool {
	return p.knownTxs.Contains(hash)
}

// HasTxPackage tells whether current peer knows the block
func (p *Peer) HasTxPackage(hash common.Hash) bool {
	return p.knownTxPackages.Contains(hash)
}

// SendNewBlock propagates an entire block to a remote peer.
func (p *Peer) SendNewBlock(block *types.Block) error {
	p.knownBlocks.Add(block.FullHash())
	return p2p.Send(p.rw, protocol.NewBlockMsg, []protocol.NewBlockData{{Block: block}})
}

// RequestOneBlock is a wrapper around the block query functions to fetch a
// single block. It is used solely by the fetcher.
func (p *Peer) RequestOneBlock(hash common.Hash) error {
	p.Log().Debug("Fetching single block", "Hash", hash)
	return p2p.Send(p.rw, protocol.GetBlocksMsg, &protocol.GetBlocksData{OriginHash: hash, Depth: uint64(1), Reverse: false, RoundFrom: 0, RoundTo: 0})
}

// RequestBlocksByHash fetches a batch of blocks corresponding to the
// specified block query, based on the Hash of an origin block.
func (p *Peer) RequestBlocksByHash(origin common.Hash, amount int, reverse bool) error {
	p.Log().Debug("Fetching batch of blocks", "count", amount, "from-Hash", origin, "reverse", reverse)
	return p2p.Send(p.rw, protocol.GetBlocksMsg, &protocol.GetBlocksData{OriginHash: origin, Depth: uint64(amount), Reverse: reverse, RoundFrom: 0, RoundTo: 0})
}

// RequestBlocksByRoundRange fetches a batch of blocks corresponding to the
// specified block query, based on the Round of an origin block.
func (p *Peer) RequestBlocksByRoundRange(roundFrom uint64, roundTo uint64) error {
	p.Log().Debug("Fetching batch of blocks", "roundFrom", roundFrom, "roundTo", roundTo)
	return p2p.Send(p.rw, protocol.GetBlocksMsg, &protocol.GetBlocksData{RoundFrom: roundFrom, RoundTo: roundTo, OriginHash: common.Hash{}, Depth: uint64(0), Reverse: false})
}

// SendBlocks sends a batch of blocks to the remote peer.
func (p *Peer) SendBlocks(blocks types.Blocks) error {
	return p2p.Send(p.rw, protocol.BlocksMsg, blocks)
}

// SendTransactions sends transactions to the peer and includes the hashes
// in its transaction Hash set for future reference.
func (p *Peer) SendTransactions(txs types.Transactions) error {
	for _, tx := range txs {
		p.knownTxs.Add(tx.Hash())
	}
	return p2p.Send(p.rw, protocol.TxMsg, txs)
}

// SendTxPackageHash announces the availability of tx package through
// a Hash notification.
func (p *Peer) SendTxPackageHash(hash common.Hash) error {
	return p2p.Send(p.rw, protocol.TxPackageHashMsg, hash)
}

// RequestTxPackage is a wrapper around the tx package query functions to fetch a
// single tx package.
func (p *Peer) RequestTxPackage(hash common.Hash) error {
	p.Log().Info("Fetching tx package", "Hash", hash)
	return p2p.Send(p.rw, protocol.GetTxPackageMsg, hash)
}

// SendTxPackage sends tx package to the peer and includes the hashes
// in its tx package Hash set for future reference.
func (p *Peer) SendTxPackage(pkg *types.TxPackage) error {
	return p2p.Send(p.rw, protocol.TxPackageMsg, pkg)
}

// RequestNodeData fetches a batch of arbitrary data from a node's known state
// data, corresponding to the specified hashes.
func (p *Peer) RequestNodeData(hashes []common.Hash) error {
	p.Log().Info("Fetching batch of state data", "count", len(hashes))

	err := p2p.Send(p.rw, protocol.GetNodeDataMsg, hashes)
	if err != nil {
		p.Log().Error("Failed Request Node Date", "hashes", hashes, "error", err)
	}
	return err
}

// SendNodeDataRLP sends a batch of arbitrary internal data, corresponding to the
// hashes requested.
func (p *Peer) SendNodeData(data [][]byte) error {
	err := p2p.Send(p.rw, protocol.NodeDataMsg, data)
	if err != nil {
		p.Log().Error("Failed Send Node Date", "err", err)
	}

	p.Log().Info("send node data ok", "dataLen", len(data))
	return err
}

func (p *Peer) RequestSyncHashList(syncStage protocol.SyncStage, hashType protocol.SyncHashType, hashEFrom protocol.HashElem, hashETo protocol.HashElem) error {
	p.Log().Info("Request sync hash list", "stage", syncStage, "type", hashType, "hashEFrom", hashEFrom, "hashETo", hashETo)
	return p2p.Send(p.rw, protocol.SyncHashListReqMsg, protocol.SyncHashListReq{
		Stage:      syncStage,
		Type:       hashType,
		HashFrom:   hashEFrom.Hash,
		HeightFrom: hashEFrom.Height,
		HashTo:     hashETo.Hash,
		HeightTo:   hashETo.Height,
	})
}

func (p *Peer) SendSyncHashList(syncStage protocol.SyncStage, hashType protocol.SyncHashType, hashList protocol.HashElems) error {
	p.Log().Debug("Send short Hash list for sync", "stage", syncStage, "type", hashType)
	return p2p.Send(p.rw, protocol.SyncHashListResMsg, protocol.SyncHashListRsp{
		Stage:  syncStage,
		Type:   hashType,
		Hashes: hashList,
	})
}

// RequestSyncBlock fetches blocks for sync
func (p *Peer) RequestSyncPreBlocksForState(hash common.Hash) error {
	p.Log().Info("request pre blocks for state")
	return p2p.Send(p.rw, protocol.SyncPreBlocksForStateReqMsg, hash)
}

//
func (p *Peer) SendSyncPreBlocksForState(blocks []*types.Block, pkgs []*types.TxPackage) error {
	p.Log().Debug("send pre blocks for state")
	return p2p.Send(p.rw, protocol.SyncPreBlocksForStateRspMsg, protocol.FetchBlockRsp{
		Blocks:     blocks,
		TxPackages: pkgs,
	})
}

func (p *Peer) RequestSyncPostBlocksForState(hashEFrom protocol.HashElem, hashETo protocol.HashElem) error {
	p.Log().Debug("Sync post blocks for state")
	return p2p.Send(p.rw, protocol.SyncPostBlocksForStateReqMsg, protocol.IntervalHashReq{HashEFrom: hashEFrom, HashETo: hashETo})
}

func (p *Peer) SendSyncPostBlocksForState(blocks types.Blocks, pkgs []*types.TxPackage, round uint64, finished bool) error {
	p.Log().Debug("send blocks for state")
	return p2p.Send(p.rw, protocol.SyncPostBlocksForStateRspMsg, protocol.FetchBlockRsp{
		Blocks:     blocks,
		TxPackages: pkgs,
		RoundTo:    round,
		Finished:   finished,
	})
}

func (p *Peer) RequestSyncPkgs(stage protocol.SyncStage, hashes []common.Hash) error {
	log.Debug("Request packages", "stage", stage, "length", len(hashes))
	return p2p.Send(p.rw, protocol.GetPkgsForBlockSyncMsg, protocol.SyncPkgsReq{
		Stage:     stage,
		PkgHashes: hashes,
	})
}

func (p *Peer) SendSyncPkgs(fetchPkgsRsp protocol.SyncPkgsRsp) error {
	p.Log().Debug("Send pkgs for sync")
	return p2p.Send(p.rw, protocol.PkgsForBlockSyncMsg, fetchPkgsRsp)
}

func (p *Peer) RequestSyncBlocks(stage protocol.SyncStage, roundFrom uint64, roundTo uint64) error {
	p.Log().Debug("Fetching batch of blocks with pkg", "roundFrom", roundFrom, "roundTo", roundTo)
	return p2p.Send(p.rw, protocol.GetBlocksForBlockSyncMsg, &protocol.SyncBlocksReq{
		Stage:     stage,
		RoundFrom: roundFrom,
		RoundTo:   roundTo,
	})
}

func (p *Peer) SendSyncBlocks(stage protocol.SyncStage, blocks types.Blocks, roundFrom uint64, roundTo uint64) error {
	p.Log().Debug("send blocks for sync")
	return p2p.Send(p.rw, protocol.BlocksForBlockSyncMsg, protocol.SyncBlocksRsp{
		Stage:     stage,
		Blocks:    blocks,
		RoundFrom: roundFrom,
		RoundTo:   roundTo,
	})
}

// Handshake executes the ftl protocol handshake, negotiating version number,
// network IDs, difficulties, head and genesis blocks.
func (p *Peer) Handshake(network uint64, height uint64, round uint64, head common.Hash, simpleHash common.Hash, genesis common.Hash) error {
	// Send out own handshake in a new thread
	errc := make(chan error, 2)
	var status protocol.StatusData // safe to read after two values have been received from errc

	go func() {
		errc <- p2p.Send(p.rw, protocol.StatusMsg, &protocol.StatusData{
			ProtocolVersion:   uint32(p.version),
			NetworkId:         network,
			Height:            height,
			Round:             round,
			CurrentFullHash:   head,
			CurrentSimpleHash: simpleHash,
			GenesisHash:       genesis,
		})
	}()
	go func() {
		errc <- p.readStatus(network, &status, genesis)
	}()
	timeout := time.NewTimer(handshakeTimeout)
	defer timeout.Stop()
	for i := 0; i < 2; i++ {
		select {
		case err := <-errc:
			if err != nil {
				return err
			}
		case <-timeout.C:
			return p2p.DiscReadTimeout
		}
	}
	p.headHeight, p.headRound, p.headFullHash, p.headSimpleHash = status.Height, status.Round, status.CurrentFullHash, status.CurrentSimpleHash
	return nil
}

func (p *Peer) readStatus(network uint64, status *protocol.StatusData, genesis common.Hash) (err error) {
	msg, err := p.rw.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != protocol.StatusMsg {
		return errResp(protocol.ErrNoStatusMsg, "first msg has code %x (!= %x)", msg.Code, protocol.StatusMsg)
	}
	if msg.Size > protocol.ProtocolMaxMsgSize {
		return errResp(protocol.ErrMsgTooLarge, "%v > %v", msg.Size, protocol.ProtocolMaxMsgSize)
	}
	// Decode the handshake and make sure everything matches
	if err := msg.Decode(&status); err != nil {
		return errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
	}
	if status.GenesisHash != genesis {
		return errResp(protocol.ErrGenesisBlockMismatch, "%x (!= %x)", status.GenesisHash[:8], genesis[:8])
	}
	if status.NetworkId != network {
		return errResp(protocol.ErrNetworkIdMismatch, "%d (!= %d)", status.NetworkId, network)
	}
	if int(status.ProtocolVersion) != p.version {
		return errResp(protocol.ErrProtocolVersionMismatch, "%d (!= %d)", status.ProtocolVersion, p.version)
	}
	return nil
}

// String implements fmt.Stringer.
func (p *Peer) String() string {
	return fmt.Sprintf("Peer %s [%s]", p.id,
		fmt.Sprintf("ftl/%2d", p.version),
	)
}

func (p *Peer) GetPeer() *Peer {
	return p
}

func (p *Peer) Name() string {
	return p.id
}
