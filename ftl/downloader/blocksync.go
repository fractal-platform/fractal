package downloader

import (
	"fmt"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
)

var downloaderNo = 1

type blockchain interface {
	HasTxPackage(hash common.Hash) bool
	IsTxPackageInFuture(hash common.Hash) bool
}

func StartFetchBlocksByHash(reqs []common.Hash, peers map[string]FetcherPeer,
	dropPeerFn peerDropFn, autoStop bool, stage protocol.SyncStage, chain blockchain, blockCh chan *types.Block) *BlockFetcherByHash {

	// create a new sub logger
	logger := log.NewSubLogger("m", fmt.Sprintf("downloader%d", downloaderNo))
	downloaderNo += 1
	logger.Info("Start Fetch Block with pkgs", "reqs", len(reqs), "peers", len(peers), "autoStop", autoStop, "stage", stage)

	// init peers manager for fetcher
	peersManager := newPeersManager(dropPeerFn)
	for _, p := range peers {
		err := peersManager.initRegisterPeer(p)
		if err != nil {
			logger.Error("Can not register the peer", "peer", p.GetID(), "error", err)
			return nil
		}
	}

	// create fetchers
	blockFetcher := newBlocksFetcherByHash(reqs, chain, peersManager, autoStop, stage, blockCh, logger)
	blockFetcher.start()
	return blockFetcher
}

func StartFetchBlocksByRound(roundFrom, roundTo uint64, peers map[string]FetcherPeer,
	dropPeerFn peerDropFn, autoStop bool, stage protocol.SyncStage, chain blockchain, blockCh chan *types.Block) *BlockFetcherByRound {

	// create a new sub logger
	logger := log.NewSubLogger("m", fmt.Sprintf("downloader%d", downloaderNo))
	downloaderNo += 1
	logger.Info("Start Fetch Block with pkgs", "roundFrom", roundFrom, "roundTo", roundTo, "peers", len(peers), "autoStop", autoStop, "stage", stage)

	// init peers manager for fetcher
	peersManager := newPeersManager(dropPeerFn)
	for _, p := range peers {
		err := peersManager.initRegisterPeer(p)
		if err != nil {
			logger.Error("Can not register the peer", "peer", p.GetID(), "error", err)
			return nil
		}
	}

	// create fetchers
	blockFetcher := newBlocksFetcherByRound(roundFrom, roundTo, chain, peersManager, autoStop, stage, blockCh, logger)
	blockFetcher.start()
	return blockFetcher
}
