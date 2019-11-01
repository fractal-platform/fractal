// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package network contains the implementation of network protocol handler for fractal.
package network

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

// HandleReturn is the top status for sync process
type HandleReturn int

const (
	HandleReturnBegin HandleReturn = iota
	HandleReturnDone
	HandleReturnIgnore
	HandleReturnEnd
)

func (s HandleReturn) String() string {
	if s <= HandleReturnBegin || s >= HandleReturnEnd {
		return "Unknown"
	}

	list := [...]string{
		"Done",
		"Ignore"}
	return list[s-1]
}

type handler interface {
	handleMsg(p *Peer, msg p2p.Msg) (HandleReturn, error)
}

type defaultHandler struct {
	pm           *ProtocolManager
	chain        blockchain
	packer       packer
	synchronizer synchronizer
	txPool       pool.Pool
	logger       log.Logger

	// for txpkg fetch
	txpkgFetcher *txpkgFetcher
}

func newDefaultHandler(pm *ProtocolManager, chain blockchain, synchronizer synchronizer, packer packer, txPool pool.Pool, logger log.Logger, txpkgFetcher *txpkgFetcher) *defaultHandler {
	h := &defaultHandler{
		pm:           pm,
		chain:        chain,
		packer:       packer,
		synchronizer: synchronizer,
		txPool:       txPool,
		logger:       logger,

		// for txpkg fetch
		txpkgFetcher: txpkgFetcher,
	}

	return h
}

func (h *defaultHandler) handleMsg(p *Peer, msg p2p.Msg) (HandleReturn, error) {
	switch {
	case msg.Code == protocol.StatusMsg:
		// Status messages should never arrive after the handshake
		return HandleReturnDone, errResp(protocol.ErrExtraStatusMsg, "uncontrolled status message")

	case msg.Code == protocol.NewBlockMsg:
		// Retrieve and decode the propagated block
		var block types.Block
		if err := msg.Decode(&block); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		// Mark the peer as owning the block and schedule it for import
		p.MarkBlock(block.FullHash())

		if h.synchronizer.IsSyncStatusNormal() {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockMined
			block.Header.HopCount++
			h.logger.Info("send to block process channel(NewBlockMsg)", "hash", block.FullHash())
			h.pm.BlockProcessCh <- &BlockWithVerifyFlag{&block, true}
		}

	case msg.Code == protocol.BlockReqMsg:
		// Decode the complex block query
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		block := h.chain.GetBlock(hash)
		if block != nil {
			err := p.SendBlock(block)
			return HandleReturnDone, err
		}
		return HandleReturnDone, nil

	case msg.Code == protocol.BlockRspMsg:
		// A batch of headers arrived to one of our previous requests
		var block types.Block
		if err := msg.Decode(&block); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}

		// Mark the peer as owning the block and schedule it for import
		p.MarkBlock(block.FullHash())

		if h.synchronizer.IsSyncStatusNormal() {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			// Fast sync starts all execution after pulling all the blocks and packages at the beginning, so there is no dependency problem. So it won't be fast sync here.
			block.ReceivedPath = types.BlockMined
			log.Info("send to block process channel(BlockRspMsg)", "hash", block.FullHash())
			h.pm.BlockProcessCh <- &BlockWithVerifyFlag{&block, true}
		}

	case msg.Code == protocol.TxMsg:
		// Transactions can be processed, parse all of them and deliver to the pool
		var txs []*types.Transaction
		if err := msg.Decode(&txs); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}

		elems := make([]pool.Element, len(txs))
		for i, tx := range txs {
			// Validate and mark the remote transaction
			if tx == nil {
				return HandleReturnDone, errResp(protocol.ErrDecode, "transaction %d is nil", i)
			}
			p.MarkTransaction(tx.Hash())
			elems[i] = tx
		}
		h.txPool.AddRemotes(elems)

	case msg.Code == protocol.TxPackageHashMsg:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		// Mark the Hash as present at the remote node
		p.MarkTxPackage(hash)

		if h.synchronizer.IsSyncStatusNormal() {
			if !h.chain.HasTxPackage(hash) && !h.chain.IsTxPackageInFuture(hash) {
				h.txpkgFetcher.insertTask(hash)
			}
		}

	case msg.Code == protocol.TxPackageReqMsg:
		var hash common.Hash
		if err := msg.Decode(&hash); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		p.Log().Info("Receive a request for tx package", "hash", hash)
		pkg := h.chain.GetTxPackage(hash)
		if pkg != nil {
			p.AsyncSendTxPackage(pkg)
		}

	case msg.Code == protocol.TxPackageRspMsg:
		var pkg types.TxPackage
		pkg.ReceivedAt = msg.ReceivedAt
		pkg.ReceivedFrom = p
		if err := msg.Decode(&pkg); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}
		p.MarkTxPackage(pkg.Hash())

		if h.synchronizer.IsSyncStatusNormal() {
			go func() {
				(&pkg).IncreaseHopCount()
				h.pm.insertTxPackage(&pkg, true, true)

				// tell fetcher
				h.txpkgFetcher.finishTask(p)
			}()
		}
	default:
		return HandleReturnIgnore, nil
	}
	return HandleReturnDone, nil
}

type syncHandler struct {
	pm           *ProtocolManager
	chain        blockchain
	synchronizer synchronizer
	logger       log.Logger
}

func newSyncHandler(pm *ProtocolManager, chain blockchain, synchronizer synchronizer, logger log.Logger) *syncHandler {
	return &syncHandler{
		pm:           pm,
		chain:        chain,
		synchronizer: synchronizer,
		logger:       logger,
	}
}

func (h *syncHandler) handleMsg(p *Peer, msg p2p.Msg) (HandleReturn, error) {

	// Handle the message depending on its contents
	switch {
	case msg.Code == protocol.NodeDataReqMsg:
		// Decode the retrieval message
		log.Info("Receive GetNodeDateMsg")
		msgStream := rlp.NewStream(msg.Payload, uint64(msg.Size))
		if _, err := msgStream.List(); err != nil {
			log.Error("Bad message")
			return HandleReturnDone, err
		}
		// Gather state data until the fetch or network limits is reached
		var (
			hash  common.Hash
			bytes int64
			data  [][]byte
		)
		for bytes < softResponseLimit && len(data) < downloader.MaxStateFetch {
			// Retrieve the Hash of the next state entry
			if err := msgStream.Decode(&hash); err == rlp.EOL {
				log.Info("rlp: end of list")
				break
			} else if err != nil {
				return HandleReturnDone, errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
			}
			// Retrieve the requested state entry, stopping if enough was found
			if entry, err := h.chain.TrieNode(hash); err == nil {
				data = append(data, entry)
				bytes += int64(len(entry))
			} else {
				log.Error("failed to fetch trie node", "hash", hash, "err", err)
				return HandleReturnDone, err
			}
		}
		return HandleReturnDone, p.SendNodeData(data, bytes)

	case msg.Code == protocol.NodeDataRspMsg:
		log.Info("receive NodeDateMsg from", "peer", p.Name())

		// A batch of node state data arrived to one of our previous requests
		var data [][]byte
		if err := msg.Decode(&data); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "msg %v: %v", msg, err)
		}
		h.synchronizer.ProcessNodeDataRsp(p, data)

	case msg.Code == protocol.SyncHashListReqMsg:
		var req protocol.SyncHashListReq
		if err := msg.Decode(&req); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		h.logger.Info("Receive sync hash list request", "peer", p.GetID(), "reqID", req.ReqID, "req", req)
		h.synchronizer.HandleHashesRequest(p, req)

	case msg.Code == protocol.SyncHashListRspMsg:
		var rsp protocol.SyncHashListRsp
		if err := msg.Decode(&rsp); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		h.logger.Info("Receive sync hash list response", "peer", p.GetID(), "reqID", rsp.ReqID, "stage", rsp.Stage, "type", rsp.Type, "hashes", len(rsp.Hashes))
		h.synchronizer.HandleHashesResponse(p, rsp)
	case msg.Code == protocol.SyncHashTreeReqMsg:
		var req protocol.SyncHashTreeReq
		if err := msg.Decode(&req); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		h.logger.Info("Receive sync hash tree request", "peer", p.GetID(), "reqID", req.ReqID, "req", req)
		h.synchronizer.HandleHashTreeRequest(p, req)

	case msg.Code == protocol.SyncHashTreeRspMsg:
		var rsp protocol.SyncHashTreeRsp
		if err := msg.Decode(&rsp); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

		h.logger.Info("Receive sync hash tree response", "peer", p.GetID(), "reqID", rsp.ReqID)
		h.synchronizer.HandleHashTreeResponse(p, rsp)

	case msg.Code == protocol.SyncBestPeerBlocksReqMsg:
		var hashReq protocol.IntervalHashReq

		if err := msg.Decode(&hashReq); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		return HandleReturnDone, h.synchronizer.ProcessBestPeerBlocksReq(p, hashReq)

	case msg.Code == protocol.SyncBestPeerBlocksRspMsg:
		var rsp protocol.FetchBlockRsp
		if err := msg.Decode(&rsp); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("Receive best peer blocks", "peer", p.Name(), "blockes", len(rsp.Blocks), "ToRound", rsp.RoundTo, "pkgs", len(rsp.TxPackages), "finnished", rsp.Finished)

		for _, pkg := range rsp.TxPackages {
			pkg.ReceivedAt = msg.ReceivedAt
			pkg.ReceivedFrom = p

			p.MarkTxPackage(pkg.Hash())
			h.pm.insertTxPackage(pkg, false, true)
		}

		for _, block := range rsp.Blocks {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockFastSync
		}

		if len(rsp.Blocks) > 0 {
			h.synchronizer.ProcessBestPeerBlocksRsp(p, rsp.Blocks)
		}
		if rsp.Finished {
			//Tell synchronizer transfer finished
			h.synchronizer.ProcessBestPeerBlocksRsp(p, types.Blocks{})
		}

	case msg.Code == protocol.PkgsForBlockSyncReqMsg:
		var query protocol.SyncPkgsReq
		if err := msg.Decode(&query); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		h.synchronizer.ProcessTxPackagesReq(p, query.ReqID, query.Stage, query.PkgHashes)

	case msg.Code == protocol.PkgsForBlockSyncRspMsg:
		var rsp protocol.SyncPkgsRsp
		if err := msg.Decode(&rsp); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("Recv pkgs msg for sync", "peer", p.Name(), "length", len(rsp.Pkgs))

		for _, pkg := range rsp.Pkgs {
			pkg.ReceivedFrom = p
			pkg.ReceivedAt = msg.ReceivedAt

			p.MarkTxPackage(pkg.Hash())
			h.pm.insertTxPackage(pkg, false, true)
		}
		h.synchronizer.ProcessTxPackagesRsp(p, rsp.ReqID, rsp.Stage, rsp.Pkgs)

	case msg.Code == protocol.BlocksForBlockSyncReqMsg:
		var query protocol.SyncBlocksReq

		if err := msg.Decode(&query); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		if err := h.synchronizer.ProcessBlocksReq(p, query.ReqID, query.Stage, query.HashReqs, query.RoundFrom, query.RoundTo); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}

	case msg.Code == protocol.BlocksForBlockSyncRspMsg:
		var rsp protocol.SyncBlocksRsp
		if err := msg.Decode(&rsp); err != nil {
			return HandleReturnDone, errResp(protocol.ErrDecode, "%v: %v", msg, err)
		}
		log.Info("receive blocks for block sync", "peer", p.Name(), "reqID", rsp.ReqID, "stage", rsp.Stage, "blocks", len(rsp.Blocks), "roundFrom", rsp.RoundFrom, "roundTo", rsp.RoundTo)

		for _, block := range rsp.Blocks {
			block.ReceivedAt = msg.ReceivedAt
			block.ReceivedFrom = p
			block.ReceivedPath = types.BlockFastSync
		}
		if rsp.Blocks == nil {
			h.synchronizer.ProcessBlocksRsp(p, rsp.ReqID, rsp.Stage, types.Blocks{})
		} else {
			h.synchronizer.ProcessBlocksRsp(p, rsp.ReqID, rsp.Stage, rsp.Blocks)
		}

	default:
		return HandleReturnIgnore, nil
	}
	return HandleReturnDone, nil
}
