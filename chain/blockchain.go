// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package chain contains implementations for basic chain operations.
package chain

import (
	"errors"
	"sync"
	"sync/atomic"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
	ftl_metrics "github.com/fractal-platform/fractal/utils/metrics"
	"github.com/hashicorp/golang-lru"
	"github.com/rcrowley/go-metrics"
)

var (
	blockInsertTimer             = metrics.NewRegisteredTimer("chain/inserts", nil)
	transactionInsertMeter       = ftl_metrics.NewRegisteredMAMeter("transaction/inserts", nil)
	blockNetWorkDelayHistogram   = metrics.NewRegisteredHistogram("block/delay", nil, metrics.NewExpDecaySample(256, 0.015))
	blockInsertRevDelayHistogram = metrics.NewRegisteredHistogram("block/InsertRevDelay", nil, metrics.NewExpDecaySample(256, 0.015))
	blockInsertHopCountHistogram = metrics.NewRegisteredHistogram("block/BlockHopCount", nil, metrics.NewExpDecaySample(256, 0.015))

	ErrNoGenesis = errors.New("Genesis not found in chain")

	ErrPackTxNotMeetSharding = errors.New("Packed transaction doesn't meet sharding rules")

	ErrPackTxSignError = errors.New("Packed transaction sign error")

	ErrMoreThanOneGenesis = errors.New("There are two different blocks with a height of 0")

	ErrConfirmUnknownBlock = errors.New("Confirm an unknown block")

	ErrBlockNotMeetGreedy = errors.New("Block doesn't meet greedy rules")

	ErrConfirmBlockNotMeetGreedy = errors.New("Confirmed block doesn't meet greedy rules")

	ErrConfirmBlockNotMeetRound = errors.New("Confirmed block doesn't meet round range")

	ErrNotConfirmParentBlock = errors.New("Not confirm parent block")

	ErrBlockHeightTooLow = errors.New("block height is too low, we skip it")

	ErrBlockNotFound = errors.New("Block not found")

	ErrCannotFindParentBlock = errors.New("Can't find parent block")

	ErrCannotFindGrandparentBlock = errors.New("Can't find grandparent block")

	ErrBlockStateError = errors.New("Block state error")

	ErrBlockStateNotFound = errors.New("Block state not Found")

	ErrBlockHeightError = errors.New("Block height error")

	ErrBlockRoundTooLow = errors.New("The block round is too low")

	ErrBlockConsensusError = errors.New("Block consensus error")

	ErrBlockTxHashError = errors.New("Block txHash error")

	ErrBlockBloomError = errors.New("Block bloom error")

	ErrBlockReceiptError = errors.New("Block receipt error")

	ErrBlockNonceInfoMissing = errors.New("Block nonce info missing")

	ErrBlockTxPackageMissing = errors.New("Block txPackage missing")

	ErrBlockSigError = errors.New("Block sig error")

	ErrBlockFullSigError = errors.New("Block full sig error")

	ErrPackageHeightTooLow = errors.New("Package height too low")

	ErrPackageHeightTooHigh = errors.New("Package height too high")

	ErrTransactionNotMatchPacker = errors.New("the transaction and the packer don't match")

	ErrPackerNotAllowed = errors.New("the packer not allowed")

	ErrIsBroadcastTx = errors.New("the transaction should be broadcast")

	ErrPackerInfoNotFound = errors.New("cannot find the packer info")

	ErrPackerNumberIsZero = errors.New("packer number is zero")

	ErrInvalidGasLimit = errors.New("invalid gas limit")

	ErrInvalidGasUsed = errors.New("invalid gas used")

	ErrTxPackageRelatedBlockNotFound = errors.New("tx package related block not found")

	ErrConfirmedBlockHasSameSimpleHash = errors.New("confirmed block hash same simple hash")
)

const (
	// Minimum height delay of the transaction package entering the block
	MinPackageHeightDelay = 2

	// Maximum height delay of the transaction package entering the block
	MaxPackageHeightDelay = 4
)

type BlockChain struct {
	logger      log.Logger
	chainConfig *config.ChainConfig
	db          dbwrapper.Database // Low level persistent database to store final content in

	// for block in blockchain
	genesisBlock *types.Block
	checkPoints  *config.CheckPoints // checkPoints
	//fixPoint         atomic.Value        // FixPoint for last fast sync
	currentBlock     atomic.Value // Current head block of the block chain
	blockCache       *lru.Cache   // Cache for the most recent block
	mainBranchRecord *MainBranchRecord

	// for state in blockchain
	stateCache state.Database // State database to reuse between imports (contains state cache)

	// for pkg in blockchain
	pkgCache           *lru.Cache
	pkgSigner          types.PkgSigner
	packerInfoMapCache *types.PackerInfoMapCache

	// for tx in blockchain
	txSigner           types.Signer
	txExecutor         txexec.TxExecutor
	txInChainProcessor *TxInChainProcessor

	// for depend process
	futureBlocks               map[common.Hash]*types.Blocks // confirmed(parent) block hash -> future block
	futureBlocksMutex          sync.RWMutex
	futureTxPackageBlocks      map[common.Hash]*types.Blocks // tx package hash -> future block
	futureTxPackageBlocksMutex sync.RWMutex
	futureBlockTxPackages      map[common.Hash]*types.TxPackages // block hash in tx package -> future tx packages
	futureBlockTxPackagesMutex sync.RWMutex

	// feed
	chainUpdateFeed   event.Feed
	blockExecutedFeed event.Feed

	// mutex
	mu sync.RWMutex // global mutex for locking chain operations
}

// NewBlockChain returns a fully initialised block chain using information
// available in the database.
func NewBlockChain(cfg *config.Config, db dbwrapper.Database, executor txexec.TxExecutor, checkPoints *config.CheckPoints, packerInfoCacheSize uint8) (*BlockChain, error) {
	logger := log.NewSubLogger("m", "blockchain")

	// create pkg cache
	pkgCache, err := lru.New(cfg.PkgCacheSize)
	if err != nil {
		logger.Error("Create pkg cache failed", "err", err)
		return nil, err
	}

	// create block cache
	blockCache, err := lru.New(1024)
	if err != nil {
		logger.Error("Create block cache failed", "err", err)
		return nil, err
	}

	// create packer info cache
	packerInfoMapCache, err := types.NewPackerInfoMapCache(packerInfoCacheSize)
	if err != nil {
		logger.Error("Create packer info map cache failed", "err", err)
		return nil, err
	}

	//
	bc := &BlockChain{
		logger:      logger,
		chainConfig: cfg.ChainConfig,
		db:          db,

		checkPoints: checkPoints,
		blockCache:  blockCache,

		stateCache: state.NewDatabase(db),

		pkgCache:           pkgCache,
		pkgSigner:          types.MakePkgSigner(false),
		packerInfoMapCache: packerInfoMapCache,

		txSigner:   types.MakeSigner(cfg.ChainConfig.TxSignerType, cfg.ChainConfig.ChainID),
		txExecutor: executor,

		futureBlocks:          make(map[common.Hash]*types.Blocks),
		futureTxPackageBlocks: make(map[common.Hash]*types.Blocks),
		futureBlockTxPackages: make(map[common.Hash]*types.TxPackages),
	}

	// set genesis block
	genesisHash := dbaccessor.ReadGenesisBlockHash(bc.db)
	if genesisHash == (common.Hash{}) {
		return nil, ErrNoGenesis
	}
	bc.genesisBlock = bc.GetBlock(genesisHash)

	// set current block
	headBlockHash := dbaccessor.ReadHeadBlockHash(bc.db)
	headBlock := bc.GetBlock(headBlockHash)
	bc.currentBlock.Store(headBlock)

	// TODO: scan chaindb

	//
	bc.mainBranchRecord = NewMainBranchRecord(bc)
	bc.mainBranchRecord.Start()

	//
	if cfg.TxSaveProcessInterval <= 0 {
		cfg.TxSaveProcessInterval = 2
	}
	bc.txInChainProcessor = NewTxInChainProcessor(bc, cfg.TxSaveProcessInterval)

	logger.Info("Init blockchain OK", "type", "console", "genesis", genesisHash, "head", headBlockHash)
	return bc, nil
}

func (bc *BlockChain) SendBlockExecutedFeed(block *types.Block) {
	bc.blockExecutedFeed.Send(types.BlockExecutedEvent{Block: block})
}

// calcAndCheckState calculate the state for block, and return whether the state is ok
func (bc *BlockChain) calcAndCheckState(block *types.Block) bool {
	if block == nil {
		return false
	}

	// get the block list for state calc
	var checkBlocks []*types.Block
	var checkBlock = block
	for {
		stateCheckedEnum := bc.GetBlockStateChecked(checkBlock)
		// BlockStateChecked
		if stateCheckedEnum == types.BlockStateChecked {
			break
		}

		// HasBlockStateButNotChecked
		if stateCheckedEnum == types.HasBlockStateButNotChecked {
			checkBlocks = append([]*types.Block{checkBlock}, checkBlocks...)
			break
		}

		// NoBlockState
		checkBlocks = append([]*types.Block{checkBlock}, checkBlocks...)
		// process parent
		parent := bc.GetBlock(checkBlock.Header.ParentFullHash)
		if parent == nil {
			return false
		}
		checkBlock = parent
	}

	// calc state and check
	for _, checkBlock := range checkBlocks {
		// Quickly validate the header and propagate the block if it passes
		var confirmBlocks types.Blocks
		for _, fullHash := range checkBlock.Header.Confirms {
			var confirmBlock = bc.GetBlock(fullHash)
			confirmBlocks = append(confirmBlocks, confirmBlock)
		}
		state, receipts, executedTxs, bloom, err := bc.execBlock(checkBlock, confirmBlocks)
		if err != nil {
			return false
		} else {
			bc.insertBlockState(checkBlock, state, receipts, executedTxs, bloom)
		}
	}
	return true
}

//
func (bc *BlockChain) checkAndSetHead(block *types.Block) {
	// calc current blocks
	currentBlock := bc.currentBlock.Load().(*types.Block)
	if block.CompareByHeightAndRoundAndSimpleHash(currentBlock) > 0 {
		comPreFixBlock := bc.findCommonPreFix(block, currentBlock)
		if bc.calcAndCheckState(block) && bc.checkCheckPoint(currentBlock, comPreFixBlock) {
			log.Info("Switch head block",
				"oldHash", currentBlock.FullHash(), "oldHeight", currentBlock.Header.Height, "oldRound", currentBlock.Header.Round,
				"newHash", block.FullHash(), "newHeight", block.Header.Height, "newRound", block.Header.Round)
			currentBlock = block
		}
	}
	bc.currentBlock.Store(currentBlock)
	dbaccessor.WriteHeadBlockHash(bc.db, currentBlock.FullHash())
}

// GetBlockStateChecked return the state checked flag
func (bc *BlockChain) GetBlockStateChecked(block *types.Block) types.BlockStateCheckedEnum {
	if block.StateChecked == types.BlockStateChecked {
		return types.BlockStateChecked
	}
	value := dbaccessor.ReadBlockStateCheck(bc.db, block.FullHash())
	if value == types.NoBlockState {
		log.Info("block state not found", "blockHeight", block.Header.Height, "fullHash", block.FullHash())
	}

	return value
}

// SetBlockState set state checked flag for block
func (bc *BlockChain) SetBlockState(block *types.Block, state types.BlockStateCheckedEnum) {
	dbaccessor.WriteBlockStateCheck(bc.db, block.FullHash(), state)
}

func (bc *BlockChain) CalcAndCheckState(block *types.Block) bool {
	return bc.calcAndCheckState(block)
}

// Genesis retrieves the chain's genesis block.
func (bc *BlockChain) Genesis() *types.Block {
	return bc.genesisBlock
}

// CurrentBlock retrieves the current head block of the canonical chain
func (bc *BlockChain) CurrentBlock() *types.Block {
	return bc.currentBlock.Load().(*types.Block)
}

// only used by sync
func (bc *BlockChain) SetCurrentBlock(currentBlock *types.Block) {
	bc.currentBlock.Store(currentBlock)
}

// HasBlock checks if a block is fully present in the database or not.
func (bc *BlockChain) HasBlock(hash common.Hash) bool {
	if bc.blockCache.Contains(hash) {
		return true
	}
	return dbaccessor.HasBlock(bc.db, hash)
}

// GetBlock retrieves a block from the database by hash,
// caching it if found.
func (bc *BlockChain) GetBlock(hash common.Hash) *types.Block {
	// Short circuit if the block's already in the cache, retrieve otherwise
	if block, ok := bc.blockCache.Get(hash); ok {
		return block.(*types.Block)
	}

	//
	header := dbaccessor.ReadBlockHeader(bc.db, hash)
	if header == nil {
		return nil
	}
	block := types.NewBlockWithHeader(header)
	block.Body = *dbaccessor.ReadBlockBody(bc.db, hash)
	block.ReceivedPath, _ = dbaccessor.ReadBlockReceivePath(bc.db, hash)

	// Cache the found block for next time and return
	bc.blockCache.Add(block.FullHash(), block)
	return block
}

func (bc *BlockChain) GetBlockChilds(hash common.Hash) []common.Hash {
	return dbaccessor.ReadBlockChilds(bc.db, hash)
}

// GetBlocksFromRoundRange retrieves the hash assigned to a round range (r1, r2]
func (bc *BlockChain) GetBlocksFromRoundRange(r1 uint64, r2 uint64) types.Blocks {
	var blocks types.Blocks

	hashList := dbaccessor.ReadHashListByRoundRange(bc.db, r1, r2)
	for _, roundHash := range hashList {
		block := bc.GetBlock(roundHash.FullHash)
		if block != nil {
			blocks = append(blocks, block)
		}
	}

	return blocks
}

// GetBlocksFromBlockRange retrieves the hash assigned to a block range (b1, b2]
func (bc *BlockChain) GetBlocksFromBlockRange(b1 *types.Block, b2 *types.Block) types.Blocks {
	var blocks types.Blocks

	hashList := dbaccessor.ReadHashListByBlockRange(bc.db, b1, b2)
	for _, roundHash := range hashList {
		block := bc.GetBlock(roundHash.FullHash)
		if block != nil {
			blocks = append(blocks, block)
		}
	}

	return blocks
}

// GetBackwardBlocks retrieves num blocks older than the block b
func (bc *BlockChain) GetBackwardBlocks(b *types.Block, num uint64) types.Blocks {
	var blocks types.Blocks

	hashList := dbaccessor.ReadHashListByBlockBackward(bc.db, b, num)
	for _, roundHash := range hashList {
		block := bc.GetBlock(roundHash.FullHash)
		if block != nil {
			blocks = append(blocks, block)
		}
	}
	blocks.SortByRoundHash()

	return blocks
}

func (bc *BlockChain) GetBlocksFromBlock(hash common.Hash, depth uint64, reverse bool) types.Blocks {
	if reverse {
		return bc.GetAncestorBlocksFromBlock(hash, depth)
	} else {
		return bc.GetDescendantBlocksFromBlock(hash, depth)
	}
}

// GetNearbyBlocksFromBlock return the block corresponding to hash and up to width-1 ancestors and
// forward to width-1 descendants.
func (bc *BlockChain) GetNearbyBlocksFromBlock(hash common.Hash, width uint64) (blocks types.Blocks) {
	next := hash
	for i := uint64(0); i < width; i++ {
		if block := bc.GetBlock(next); block != nil {
			hash, next = next, block.Header.ParentFullHash
		}
	}
	return bc.GetDescendantBlocksFromBlock(hash, width*2)
}

// GetAncestorBlocksFromBlock returns the block corresponding to hash and up to n-1 ancestors.
func (bc *BlockChain) GetAncestorBlocksFromBlock(hash common.Hash, depth uint64) (blocks types.Blocks) {
	for i := uint64(0); i < depth; i++ {
		block := bc.GetBlock(hash)
		if block == nil {
			break
		}
		blocks = append(blocks, block)
		hash = block.Header.ParentFullHash
	}
	return
}

// GetDescendantBlocksFromBlock returns the block corresponding to hash and forward to n-1 descendants.
func (bc *BlockChain) GetDescendantBlocksFromBlock(hash common.Hash, depth uint64) (blocks types.Blocks) {
	depth -= 1
	if depth > 0 {
		childs := bc.GetBlockChilds(hash)
		if len(childs) > 0 {
			for _, child := range childs {
				blocks = append(blocks, bc.GetDescendantBlocksFromBlock(child, depth)...)
			}
		}
	}
	block := bc.GetBlock(hash)
	if block != nil {
		blocks = append(blocks, block)
	}
	return
}

// SubscribeChainUpdateEvent registers a subscription of ChainUpdateEvent.
func (bc *BlockChain) SubscribeChainUpdateEvent(ch chan<- types.ChainUpdateEvent) event.Subscription {
	return bc.chainUpdateFeed.Subscribe(ch)
}

func (bc *BlockChain) SubscribeBlockExecutedEvent(ch chan<- types.BlockExecutedEvent) event.Subscription {
	return bc.blockExecutedFeed.Subscribe(ch)
}

// TrieNode retrieves a blob of data associated with a trie node (or code hash)
// either from ephemeral in-memory cache, or from persistent storage.
func (bc *BlockChain) TrieNode(hash common.Hash) ([]byte, error) {
	return bc.stateCache.TrieDB().Node(hash)
}

func (bc *BlockChain) Database() dbwrapper.Database        { return bc.db }
func (bc *BlockChain) GetChainID() uint64                  { return bc.chainConfig.ChainID }
func (bc *BlockChain) GetGreedy() uint8                    { return bc.chainConfig.Greedy }
func (bc *BlockChain) GetChainConfig() *config.ChainConfig { return bc.chainConfig }
func (bc *BlockChain) GetCheckPoints() *config.CheckPoints { return bc.checkPoints }

func (bc *BlockChain) SetMainBranchRecordBackend(m *MainBranchRecord)               { bc.mainBranchRecord = m }
func (bc *BlockChain) GetMainBranchBlock(height uint64) (*types.BlockHeader, error) { return bc.mainBranchRecord.GetMainBranchBlock(height) }
func (bc *BlockChain) IsInMainBranch(block *types.Block) bool {
	if block == nil {
		return false
	}
	header, err := bc.GetMainBranchBlock(block.Header.Height)
	if err != nil {
		return false
	}
	if header.FullHash() == block.FullHash() {
		return true
	}
	return false
}

func IsReOrg(new *types.Block, oldHead *types.Block) bool {
	if new.Header.Height < oldHead.Header.Height {
		return false
	} else if new.Header.Height == oldHead.Header.Height {
		if new.Header.Round < oldHead.Header.Round {
			return true
		} else if new.Header.Round == oldHead.Header.Round {
			return new.SimpleHash().Hex() < oldHead.SimpleHash().Hex()
		} else {
			return false
		}
	} else {
		return true
	}
}

func (bc *BlockChain) SearchTransactionInCache(txHash common.Hash) (*types.Transaction, common.Hash) {
	return bc.txInChainProcessor.SearchTransactionInHeap(txHash)
}

func (bc *BlockChain) StopRecord() {
	bc.mainBranchRecord.Stop()
	bc.txInChainProcessor.Stop()
}
