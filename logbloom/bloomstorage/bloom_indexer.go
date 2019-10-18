package bloomstorage

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/logbloom"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

var ErrCannotFindSection = errors.New("cannot find section")

// blockChain interface is used for connecting the indexer to a blockchain.
type blockChain interface {
	GetBlock(hash common.Hash) *types.Block
	CurrentBlock() *types.Block
	SubscribeBlockExecutedEvent(ch chan<- types.BlockExecutedEvent) event.Subscription
	GetMainBranchBlock(height uint64) (*types.BlockHeader, error)
}

type SectionBuildNotify struct {
	SectionNum   uint64
	SectionBloom []logbloom.OneBloom
}

// BloomIndexer does a post-processing job for equally sized sections of the
// canonical chain (like BlooomBits and CHT structures).
type BloomIndexer struct {
	chainDb dbwrapper.Database // Chain database to index the data from
	writer  *SectionWriter

	blockExecutedHead *types.Block

	active    uint32                  // Flag whether the event loop was started
	update    chan SectionBuildNotify // Notification channel that headers should be processed
	quit      chan chan error         // Quit channel to tear down running goroutines
	ctx       context.Context
	ctxCancel func()
}

// NewBloomIndexer creates a new chain indexer to do background processing on
// chain segments of a given size after certain number of confirmations passed.
func NewBloomIndexer(chainDb dbwrapper.Database) *BloomIndexer {
	c := &BloomIndexer{
		chainDb: chainDb,
		writer:  NewSectionWriter(chainDb),
		update:  make(chan SectionBuildNotify, 1),
		quit:    make(chan chan error),
	}
	// Initialize database dependent fields and start the updater
	c.ctx, c.ctxCancel = context.WithCancel(context.Background())

	go c.updateLoop()

	return c
}

// Start makes some initialization settings for the bloom, including initialization when the system is restarted.
func (b *BloomIndexer) Start(chain blockChain) {
	events := make(chan types.BlockExecutedEvent, 10)
	sub := chain.SubscribeBlockExecutedEvent(events)

	bloomInfoList := logbloom.GetBloomList()
	bloomInfoList.Mu.Lock()

	genesis, _ := chain.GetMainBranchBlock(0)
	b.blockExecutedHead = chain.GetBlock(genesis.FullHash())

	// when system restart
	if currentBlock := chain.CurrentBlock(); currentBlock != nil {
		currentSecId := currentBlock.Header.Height / params.BloomBitsSize
		// TODO: secId start from check point, not 0
		for secId := uint64(0); secId <= currentSecId; secId++ {
			// To see if this section has been saved
			saved := dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, secId)
			if saved {
				continue
			}

			// If the entire section has not been saved, then check if each item(a block) in it has been executed. If the block has been executed, put it into the buffer.
			sectionHeadHeight := secId * params.BloomBitsSize
			for i := sectionHeadHeight; i <= currentBlock.Header.Height && i < sectionHeadHeight+params.BloomBitsSize; i++ {
				if blockHeader, err := chain.GetMainBranchBlock(i); err == nil && dbaccessor.ReadBlockStateCheck(b.chainDb, blockHeader.FullHash()) == types.BlockStateChecked {
					if b.blockExecutedHead.Header.Height < blockHeader.Height {
						b.blockExecutedHead = chain.GetBlock(blockHeader.FullHash())
					}
					bloomInfoList.InsertBlockBloom(b.chainDb, blockHeader, nil)
				}
			}
			b.sendSaveBloomMapChan(bloomInfoList, secId)
		}
	}

	bloomInfoList.Mu.Unlock()

	go b.eventLoop(events, sub, chain)
}

// Close tears down all goroutines belonging to the indexer and returns any error
// that might have occurred internally.
func (b *BloomIndexer) Close() error {
	var errs []error

	b.ctxCancel()

	// Tear down the primary update loop
	errc := make(chan error)
	b.quit <- errc
	if err := <-errc; err != nil {
		errs = append(errs, err)
	}
	// If needed, tear down the secondary event loop
	if atomic.LoadUint32(&b.active) != 0 {
		b.quit <- errc
		if err := <-errc; err != nil {
			errs = append(errs, err)
		}
	}
	// Return any failures
	switch {
	case len(errs) == 0:
		return nil

	case len(errs) == 1:
		return errs[0]

	default:
		return fmt.Errorf("%v", errs)
	}
}

// eventLoop is a secondary - optional - event loop of the indexer which is only
// started for the outermost indexer to push block executed events into a processing
// queue.
func (b *BloomIndexer) eventLoop(events chan types.BlockExecutedEvent, sub event.Subscription, blockChain blockChain) {
	// Mark the chain indexer as active, requiring an additional teardown
	atomic.StoreUint32(&b.active, 1)

	defer sub.Unsubscribe()

	for {
		select {
		case errc := <-b.quit:
			// Chain indexer terminating, report no failure and abort
			errc <- nil
			return

		case ev, ok := <-events:
			// Received a new event, ensure it's not nil (closing) and update
			if !ok {
				errc := <-b.quit
				errc <- nil
				return
			}
			block := ev.Block
			blockReceivedPath := block.ReceivedPath

			bloomInfoList := logbloom.GetBloomList()

			// BlockFastSync
			if blockReceivedPath == types.BlockFastSync {
				log.Info("eventLoop: BlockFastSync", "blockHeight", block.Header.Height)

				lastBlock := blockChain.GetBlock(block.Header.ParentFullHash)
				if lastBlock == nil {
					log.Error("eventLoop: Can't find last block", "thisBlockHash", block.FullHash(), "lastBlockHash", block.Header.ParentFullHash)
					panic("eventLoop: Can't find last block")
				}
				if dbaccessor.ReadBlockStateCheck(b.chainDb, lastBlock.FullHash()) == types.BlockStateChecked {
					if b.blockExecutedHead.Header.Height < block.Header.Height {
						b.blockExecutedHead = block

						bloomInfoList.Mu.Lock()
						currentSection := bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
						b.sendSaveBloomMapChan(bloomInfoList, currentSection)
						bloomInfoList.Mu.Unlock()
					} else {
						// currentHeader.Header.Height >= block.Header.Height
						blockSectionId := block.Header.Height / params.BloomBitsSize
						if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, blockSectionId) {
							// replace db
							thisBloom := block.Bloom()
							if thisBloom == nil {
								thisBloom, _ = dbaccessor.ReadBloom(b.chainDb, block.FullHash())
							}
							replaceErr := b.writer.ReplaceSectionBit(blockSectionId, block.Header.Height, thisBloom)
							if replaceErr != nil {
								log.Error("eventLoop: Can't replace section bit", "section", blockSectionId, "height", block.Header.Height, "err", replaceErr)
							}
						} else {
							// replace cache map
							bloomInfoList.Mu.Lock()
							currentSection := bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
							b.sendSaveBloomMapChan(bloomInfoList, currentSection)
							bloomInfoList.Mu.Unlock()
						}
					}
				} else { // must be the fix point
					if b.blockExecutedHead.Header.Height < block.Header.Height {
						b.blockExecutedHead = block

						bloomInfoList.Mu.Lock()
						currentSection := bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
						b.sendSaveBloomMapChan(bloomInfoList, currentSection)
						bloomInfoList.Mu.Unlock()
					}
					// TODO: else: clear later sections
				}
				continue
			}

			// BlockMined
			if b.blockExecutedHead.SimpleHash() == block.Header.ParentHash {
				// main chain head
				b.blockExecutedHead = block
				log.Info("eventLoop: main chain head grows", "currentHeight", b.blockExecutedHead.Header.Height)

				bloomInfoList.Mu.Lock()
				currentSection := bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
				b.sendSaveBloomMapChan(bloomInfoList, currentSection)
				bloomInfoList.Mu.Unlock()
			} else if chain.IsReOrg(block, b.blockExecutedHead) {
				// reorg will not occur when fast sync
				reorg := dbaccessor.FindReorgChain(b.chainDb, &b.blockExecutedHead.Header, &block.Header)
				if len(reorg) <= 1 {
					log.Error("eventLoop: reorg happen. find common ancestor error") // should not happen
					continue                                                         // at least have two element (common ancestor,  and another block which is more preferred than original main chain head)
				}
				b.blockExecutedHead = block
				headBlockCachedBloom := b.blockExecutedHead.Bloom()

				commonAncestor := reorg[0]
				commonAncestorHeight := commonAncestor.Height
				commonAncestorSection := commonAncestorHeight / params.BloomBitsSize

				log.Info("eventLoop: reorg happen", "commonAncestorHeight", commonAncestorHeight, "blockHeight", block.Header.Height)

				bloomInfoList.Mu.Lock()
				// 1. clear the later sections
				for i := commonAncestorSection + 1; ; i++ {
					if _, exist := bloomInfoList.BListMap[i]; exist {
						log.Info("eventLoop:  clear section in map", "section", i)
						delete(bloomInfoList.BListMap, i)
					} else if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, i) {
						log.Info("eventLoop: clear section in db", "section", i)
						b.writer.ClearSection(i)
					} else {
						break
					}
				}

				// 2. reset the first section
				var leftBlock []*types.BlockHeader
				if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, commonAncestorSection) {
					var blooms []logbloom.OneBloom
					blooms, leftBlock = bloomInfoList.ResetBlockBloomDb(b.chainDb, reorg, headBlockCachedBloom)
					b.update <- SectionBuildNotify{
						SectionNum:   commonAncestorSection,
						SectionBloom: blooms,
					}
				} else {
					leftBlock = bloomInfoList.ResetBlockBloomMap(b.chainDb, reorg, headBlockCachedBloom)
					b.sendSaveBloomMapChan(bloomInfoList, commonAncestorSection)
				}

				// 3. set the later sections
				for len(leftBlock) >= int(params.BloomBitsSize) {
					currentSection := bloomInfoList.InsertBlockSectionBloom(b.chainDb, leftBlock[0:params.BloomBitsSize])
					b.sendSaveBloomMapChan(bloomInfoList, currentSection)
					leftBlock = leftBlock[params.BloomBitsSize:]
				}
				for i := 0; i < len(leftBlock)-1; i++ {
					bloomInfoList.InsertBlockBloom(b.chainDb, leftBlock[i], nil)
				}

				// 4. set the last block bloom
				if len(leftBlock) > 0 {
					bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
				}

				bloomInfoList.Mu.Unlock()
			}
		}
	}
}

// updateLoop is the main event loop of the indexer which pushes chain segments
// down into the processing backend.
func (b *BloomIndexer) updateLoop() {
	for {
		select {
		case errc := <-b.quit:
			// Bloom indexer terminating, report no failure and abort
			errc <- nil
			return

		case res := <-b.update:
			// Process the newly defined section in the background
			err := b.writer.WriteSection(res.SectionNum, res.SectionBloom)
			if err != nil {
				// If processing failed, don't retry until further notification
				log.Error("Section processing failed", "section", res.SectionNum, "err", err)
			}
		}
	}
}

// check if need to save build a section
func (b *BloomIndexer) sendSaveBloomMapChan(bloomInfoList *logbloom.BloomList, currentSection uint64) {
	currentSectionBloom := bloomInfoList.BListMap[currentSection]
	if currentSectionBloom != nil && currentSectionBloom.IsFull() {
		if currentSection >= 1 {
			if prevSectionBloom, exist := bloomInfoList.BListMap[currentSection-1]; exist && prevSectionBloom.IsFull() {
				b.update <- SectionBuildNotify{
					SectionNum:   currentSection - 1,
					SectionBloom: prevSectionBloom.Blooms[:],
				}
				delete(bloomInfoList.BListMap, currentSection-1)
			}
		}

		if nextSectionBloom, exist := bloomInfoList.BListMap[currentSection+1]; (exist && nextSectionBloom.IsFull()) || dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, currentSection+1) {
			b.update <- SectionBuildNotify{
				SectionNum:   currentSection,
				SectionBloom: currentSectionBloom.Blooms[:],
			}
			delete(bloomInfoList.BListMap, currentSection)
		}
	}
}
