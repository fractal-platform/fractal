package downloader

import (
	"fmt"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/ftl/protocol"
	"github.com/fractal-platform/fractal/utils/log"
)

var downloaderNo = 1

type blockchain interface {
	HasTxPackage(hash common.Hash) bool
	GetTxPackage(hash common.Hash) *types.TxPackage
	IsTxPackageInFuture(hash common.Hash) bool
	GetRelatedBlockForFutureTxPackage(hash common.Hash) common.Hash

	CurrentBlock() *types.Block
	SendBlockExecutedFeed(block *types.Block)
	GetBlocksFromRoundRange(r1 uint64, r2 uint64) types.Blocks
	SetCurrentBlock(currentBlock *types.Block)
	InsertBlock(block *types.Block)
	InsertPastBlock(block *types.Block) error
	InsertBlockNoCheck(block *types.Block)
	GetChainConfig() *config.ChainConfig
	HasBlock(hash common.Hash) bool
	GetBlock(hash common.Hash) *types.Block
	Database() dbwrapper.Database
	Genesis() *types.Block
	VerifyBlock(block *types.Block, checkGreedy bool) (types.Blocks, common.Hash, common.Hash, error)
	GetCheckPoints() *config.CheckPoints
	GetBreakPoint(checkpoint *types.Block, headBlock *types.Block) (*types.Block, *types.Block, error)
}

func StartFetchBlocks(roundFrom, roundTo uint64, peers map[string]FetcherPeer,
	dropPeerFn peerDropFn, autoStop bool, stage protocol.SyncStage, chain blockchain, blockCh chan *types.Block) *BlockFetcher {

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
	blockFetcher := newBlocksFetcher(roundFrom, roundTo, chain, peersManager, autoStop, stage, blockCh, logger)
	blockFetcher.start()
	return blockFetcher
}
