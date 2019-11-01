package sync

import (
	"errors"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/downloader"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

func (s *Synchronizer) ProcessNodeDataRsp(p *network.Peer, data [][]byte) {
	if s.stateSync != nil {
		s.stateSync.DeliverData(p.GetID(), data)
	}
}

func (s *Synchronizer) ProcessTxPackagesReq(peer *network.Peer, reqID uint64, stage protocol.SyncStage, pkgHashes []common.Hash) {
	log.Info("Recv fectch pkgs req", "peer", peer.Name(), "req", reqID, "stage", stage, "length", len(pkgHashes))

	if !s.IsSyncStatusNormal() || s.cp2fp.isRunning() {
		err := peer.SendSyncPkgs(protocol.SyncPkgsRsp{
			RequestData: protocol.RequestData{ReqID: reqID},
			Stage:       stage,
			Pkgs:        types.TxPackages{},
		}, 0)
		if err != nil {
			log.Error("Process sync req failed", "peer", peer.Name(), "err", err)
		}
		return
	}

	var pkgs []*types.TxPackage
	go func() {
		for _, pkghash := range pkgHashes {
			pkg := s.chain.GetTxPackage(pkghash)
			if pkg != nil {
				pkgs = append(pkgs, pkg)
			}
		}

		fetchPkgsRsp := protocol.SyncPkgsRsp{
			Stage: stage,
			Pkgs:  pkgs,
		}
		encoded, _ := rlp.EncodeToBytes(fetchPkgsRsp)

		size := int64(len(encoded))

		log.Info("Process sync pkgs req", "peer", peer.Name(), "pkgs", len(pkgs), "size", size)

		err := peer.SendSyncPkgs(fetchPkgsRsp, size)
		if err != nil {
			log.Error("Process sync req failed", "peer", peer.Name(), "err", err)
		}
	}()
}

func (s *Synchronizer) ProcessTxPackagesRsp(peer *network.Peer, reqID uint64, stage protocol.SyncStage, pkgs []*types.TxPackage) {
	if stage == protocol.SyncStageCP2FP {
		s.cp2fp.deliverData(peer.GetID(), pkgs, downloader.Pkgs)
	} else {
		if s.blockSync != nil {
			s.blockSync.DeliverData(peer.GetID(), pkgs, downloader.Pkgs)
		} else if s.blockSyncRound != nil {
			s.blockSyncRound.DeliverData(peer.GetID(), pkgs, downloader.Pkgs)
		}
	}
}

func (s *Synchronizer) ProcessBlocksReq(peer *network.Peer, reqID uint64, stage protocol.SyncStage, hashReq []common.Hash, roundFrom uint64, roundTo uint64) error {
	currentRound := s.chain.CurrentBlock().Header.Round
	genesisRound := s.chain.Genesis().Header.Round
	var blocks types.Blocks
	// if the peer is not in normal status or is syncing blocks from checkpoint to fix point, not provide sync server.
	if !s.IsSyncStatusNormal() || s.cp2fp.isRunning() {
		err := peer.SendSyncBlocks(stage, reqID, types.Blocks{}, roundFrom, roundTo)
		if err != nil {
			log.Error("Process sync req failed", "peer", peer.Name(), "err", err)
		}
		return nil
	}

	if len(hashReq) > 0 {
		log.Info("Recv fetch blocks req by hash", "peer", peer.Name(), "size", len(hashReq), "reqID", reqID)
		go func() {
			for _, hash := range hashReq {
				block := s.chain.GetBlock(hash)
				if block != nil {
					blocks = append(blocks, block)
				}
				err := peer.SendSyncBlocks(stage, reqID, blocks, roundFrom, roundTo)
				if err != nil {
					log.Error("Process sync req failed", "peer", peer.Name(), "err", err)
				}
			}
		}()
		return nil
	}

	log.Info("Recv fetch blocks req by round", "peer", peer.Name(), "RoundFrom", roundFrom, "RoundTo", roundTo, "currentRound", currentRound, "genesisRound", genesisRound, "reqID", reqID)

	if roundFrom > currentRound || roundFrom < genesisRound-1 {
		err := errors.New("The requested round is beyond current round or older than genesis round.")
		return err
	}

	go func() {
		if currentRound > roundTo {
			blocks = s.chain.GetBlocksFromRoundRange(roundFrom, roundTo)
		} else {
			blocks = s.chain.GetBlocksFromRoundRange(roundFrom, currentRound)
		}

		if blocks == nil {
			blocks = types.Blocks{}
		}

		log.Info("Process sync req", "peer", peer.Name(), "RoundFrom", roundFrom, "RoundTo", roundTo, "blocks", len(blocks))

		err := peer.SendSyncBlocks(stage, reqID, blocks, roundFrom, roundTo)
		if err != nil {
			log.Error("Process sync req failed", "peer", peer.Name(), "err", err)
		}
	}()
	return nil

}

func (s *Synchronizer) ProcessBlocksRsp(peer *network.Peer, reqID uint64, stage protocol.SyncStage, blocks types.Blocks) {
	if stage == protocol.SyncStageCP2FP {
		s.cp2fp.deliverData(peer.GetID(), blocks, downloader.Blocks)
	} else {
		if s.blockSync != nil {
			s.blockSync.DeliverData(peer.GetID(), blocks, downloader.Blocks)
		} else if s.blockSyncRound != nil {
			s.blockSyncRound.DeliverData(peer.GetID(), blocks, downloader.Blocks)
		}
	}
}

func (s *Synchronizer) ProcessSyncPreBlocksForStateRsp(p *network.Peer, blocks types.Blocks) {
	s.blocksForPreStateRevCh <- blocks
}

func (s *Synchronizer) ProcessBestPeerBlocksReq(p *network.Peer, hashReq protocol.IntervalHashReq) error {
	var toBlock *types.Block
	var fromBlock *types.Block
	log.Info("process best peer blocks", "peer", p.Name(), "IntervalHashReq", hashReq)
	if hashReq.HashEFrom == (protocol.HashElem{}) {
		log.Error("SyncBestPeerBlocksReqMsg", "iHash", hashReq)
		return errors.New("wrong args")
	}
	if hashReq.HashETo == (protocol.HashElem{}) {
		toBlock = s.chain.CurrentBlock()
	} else {
		toBlock = s.chain.GetBlock(hashReq.HashETo.Hash)
	}
	fromBlock = s.chain.GetBlock(hashReq.HashEFrom.Hash)
	if toBlock == nil || fromBlock == nil {
		return errors.New("can't find fromBlock and toBlock")
	}

	lastRound := toBlock.Header.Round
	roundFrom := fromBlock.Header.Round
	s.log.Info("process best peer block ", "fromBlockHash", fromBlock.FullHash(), "fromBlockHeight", fromBlock.Header.Height, "toBlockHash", toBlock.FullHash(), "toBlockHeight", toBlock.Header.Height)

	go func() {
		for {
			var pkgHashSet = mapset.NewSet()
			var pkgs []*types.TxPackage
			roundTo := roundFrom + 30
			blocks := s.chain.GetBlocksFromRoundRange(roundFrom, roundTo)
			for _, block := range blocks {
				for _, pkgHash := range block.Body.TxPackageHashes {
					if !pkgHashSet.Contains(pkgHash) {
						pkg := s.chain.GetTxPackage(pkgHash)
						if pkg != nil {
							pkgs = append(pkgs, pkg)
						}
						pkgHashSet.Add(pkgHash)
					}
				}
			}

			if len(blocks) > 0 {
				log.Info("Process sync req", "peer", p.Name(), "RoundFrom", roundFrom, "RoundTo", roundTo, "Blocks", len(blocks), "pkgs", len(pkgs), "finish", false)
				err := p.SendSyncBestPeerBLocks(blocks, pkgs, roundTo, false)
				if err != nil {
					log.Info("Process sync req failed", "peer", p.Name(), "err", err)
					p.SendSyncBestPeerBLocks(types.Blocks{}, []*types.TxPackage{}, roundTo, true)
				}
			} else if roundFrom >= lastRound {
				log.Info("Process sync req", "peer", p.Name(), "RoundTo", roundTo, "Blocks", len(blocks), "pkgs", len(pkgs), "finish", true)
				err := p.SendSyncBestPeerBLocks(blocks, pkgs, roundTo, true)
				if err != nil {
					log.Info("Process sync req finish failed", "peer", p.Name(), "err", err)
				} else {
					log.Info("Process sync req finish", "peer", p.Name())
				}
				break
			}
			roundFrom = roundFrom + 30
		}
	}()
	return nil
}
func (s *Synchronizer) ProcessBestPeerBlocksRsp(p *network.Peer, blocks types.Blocks) {
	s.blocksForPostStateRevCh <- blocks
}

func (s *Synchronizer) HandleHashesRequest(p *network.Peer, hashesReq protocol.SyncHashListReq) {
	if s.GetSyncStatus() == SyncStatusNormal && !s.cp2fp.isRunning() {
		switch hashesReq.Type {
		case protocol.SyncHashTypeShort:
			s.handleShortHashesReq(p, hashesReq)
		case protocol.SyncHashTypeLong:
			s.handleSkeletonHashesReq(p, hashesReq)
		}
	} else {
		s.log.Info("Reject hash request", "syncStatus", s.GetSyncStatus(), "cp2fp", s.cp2fp.isRunning())
		p.SendSyncHashList(hashesReq.ReqID, hashesReq.Stage, hashesReq.Type, protocol.HashElems{})
	}
}

func (s *Synchronizer) getLocalShortHashes() (protocol.HashElems, error) {
	block := s.chain.CurrentBlock()

	if block.Header.Height < uint64(s.config.ShortHashListLength-1) {
		return nil, errors.New("sorry ,don't have enough Blocks yet")
	}
	var hashList protocol.HashElems
	for i := 0; i < s.config.ShortHashListLength; i++ {
		hashList = append(hashList, &protocol.HashElem{Height: block.Header.Height, Hash: block.FullHash(), Round: block.Header.Round, AccHash: block.AccHash})
		parentBlock := s.chain.GetBlock(block.Header.ParentFullHash)
		if block == nil {
			s.log.Error("local parent block is nil", "parentHash", block.Header.ParentFullHash, "blockHash", block.FullHash(), "blockHeight", block.Header.Height)
			return nil, errors.New("block is nil")
		}
		block = parentBlock
	}

	s.log.Info("local short hashes", "len(hashList)", len(hashList))
	return hashList, nil
}

func (s *Synchronizer) handleShortHashesReq(p *network.Peer, hashesReq protocol.SyncHashListReq) error {
	block := s.chain.CurrentBlock()
	log.Info("handle short hashes req", "currentBlockHeight", block.Header.Height, "req", hashesReq, "peer", p.GetID())
	hashList, err := s.getLocalShortHashes()
	if err != nil {
		return errors.New("get short hash list failed")
	}
	return p.SendSyncHashList(hashesReq.ReqID, hashesReq.Stage, hashesReq.Type, hashList)
}

func (s *Synchronizer) longHashes(hashesReq protocol.SyncHashListReq) (protocol.HashElems, error) {
	var rep protocol.HashElems
	var blockTo, checkpoint *types.Block
	if hashesReq.HashTo == (common.Hash{}) {
		blockTo = s.chain.CurrentBlock()
		hashesReq.HashTo = blockTo.FullHash()
	} else {
		blockTo = s.chain.GetBlock(hashesReq.HashTo)
	}

	if hashesReq.HashFrom == (common.Hash{}) {
		checkpoint = s.chain.Genesis()
		hashesReq.HashFrom = s.chain.Genesis().FullHash()
	} else {
		checkpoint = s.chain.GetBlock(hashesReq.HashFrom)
	}

	// if nil or toHeight<fromHeight ,error happens
	if checkpoint == nil || blockTo == nil || blockTo.Header.Height <= checkpoint.Header.Height {
		log.Error("failed to get block", "checkpoint", checkpoint, "blockTo", blockTo, "req", hashesReq)
		return nil, errFailedGetBlock
	}

	// get long HashList with Interval
	for blockTo.FullHash() != checkpoint.FullHash() {
		rep = append(rep, &protocol.HashElem{Height: blockTo.Header.Height, Hash: blockTo.FullHash(), Round: blockTo.Header.Round, AccHash: blockTo.AccHash})
		lastBlock := blockTo
		blockTo = s.chain.GetBlock(blockTo.Header.ParentFullHash)
		if blockTo == nil {
			log.Error("can't find parent between checkpoint 2 current", "hash", lastBlock.FullHash(), "height", lastBlock.Header.Height, "round", lastBlock.Header.Round)
			return nil, errFailedGetBlock
		}
	}
	// append checkpoint block
	rep = append(rep, &protocol.HashElem{Height: blockTo.Header.Height, Hash: blockTo.FullHash(), Round: blockTo.Header.Round, AccHash: blockTo.AccHash})
	return rep, nil
}

func (s *Synchronizer) handleSkeletonHashesReq(p *network.Peer, hashesReq protocol.SyncHashListReq) error {
	s.log.Info("handle skeleton hashes req", "req", hashesReq, "peer", p.GetID(), "genesis.hash", s.chain.Genesis().FullHash())
	rep, err := s.longHashes(hashesReq)
	if err != nil {
		return err
	}
	return p.SendSyncHashList(hashesReq.ReqID, hashesReq.Stage, hashesReq.Type, rep)
}

func (s *Synchronizer) HandleHashesResponse(p *network.Peer, hashesRes protocol.SyncHashListRsp) {
	switch hashesRes.Stage {
	case protocol.SyncStageCP2FP:
		s.syncHashListChForCP2FP <- PeerHashElemList{hashesRes.Type, p, hashesRes.Hashes}
	case protocol.SyncStageFastSync:
		s.syncHashListChForFastSync <- PeerHashElemList{hashesRes.Type, p, hashesRes.Hashes}
	case protocol.SyncStagePeerSync:
		s.syncHashListChForPeerSync <- PeerHashElemList{hashesRes.Type, p, hashesRes.Hashes}
	default:
	}
}

func (s *Synchronizer) HandleHashTreeRequest(p *network.Peer, hashTreeReq protocol.SyncHashTreeReq) {
	if s.GetSyncStatus() == SyncStatusNormal && !s.cp2fp.isRunning() {
		hashTree, treePoint, err := s.chain.CreateHashTree(hashTreeReq.HashFrom, hashTreeReq.HashTo)
		if err != nil {
			s.log.Error("handle hash tree failed", "err", err, "reqID", hashTreeReq.ReqID)
			return
		}
		if err = p.SendSyncHashTree(hashTreeReq.ReqID, *hashTree, *treePoint); err != nil {
			s.log.Error("handle hash tree failed", "err", err, "reqID", hashTreeReq.ReqID)
			return
		}
	} else {
		s.log.Info("Reject hash tree request", "syncStatus", s.GetSyncStatus(), "cp2fp", s.cp2fp.isRunning())
		if err := p.SendSyncHashTree(hashTreeReq.ReqID, types.HashTree{}, types.TreePoint{}); err != nil {
			s.log.Error("handle hash tree failed", "err", err, "reqID", hashTreeReq.ReqID)
			return
		}
	}
}

func (s *Synchronizer) HandleHashTreeResponse(p *network.Peer, hashTreeRes protocol.SyncHashTreeRsp) {
	s.hashTreeRevCh <- hashTreeRes
}
