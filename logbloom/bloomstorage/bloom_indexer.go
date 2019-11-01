package bloomstorage

import (
	"context"
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

// blockChain interface is used for connecting the indexer to a blockchain.
type blockChain interface {
	GetBlock(hash common.Hash) *types.Block
	CurrentBlock() *types.Block
	SubscribeBlockExecutedEvent(ch chan<- types.BlockExecutedEvent) event.Subscription
	GetMainBranchBlock(height uint64) (*types.Block, error)
	GetCheckPoint() *types.CheckPoint
}

type sectionUpdateTypeEnum byte

const (
	build sectionUpdateTypeEnum = iota
	bloomReplace
	clear
)

type sectionUpdateNotify struct {
	updateType sectionUpdateTypeEnum

	// could be sectionBuildNotify, sectionBloomReplaceNotify or sectionClearNotify
	data interface{}
}

type sectionBuildNotify struct {
	sectionNum   uint64
	sectionBloom []logbloom.OneBloom
}

type sectionBloomReplaceNotify struct {
	sectionNum  uint64
	bloomHeight uint64
	bloomBit    *types.Bloom
}

type sectionClearNotify struct {
	sectionNum uint64
}

// BloomIndexer does a post-processing job for equally sized sections of the
// canonical chain (like BlooomBits and CHT structures).
type BloomIndexer struct {
	chainDb dbwrapper.Database // Chain database to index the data from
	writer  *SectionWriter

	blockExecutedHead *types.Block

	bloomInsertFeed event.Feed

	active    uint32                   // Flag whether the event loop was started
	update    chan sectionUpdateNotify // Notification channel that section should be processed
	quit      chan chan error          // Quit channel to tear down running goroutines
	ctx       context.Context
	ctxCancel func()
}

// NewBloomIndexer creates a new chain indexer to do background processing on
// chain segments of a given size after certain number of confirmations passed.
func NewBloomIndexer(chainDb dbwrapper.Database) *BloomIndexer {
	c := &BloomIndexer{
		chainDb: chainDb,
		writer:  NewSectionWriter(chainDb),
		update:  make(chan sectionUpdateNotify, 16),
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

		// get the first section id
		var secBegin uint64
		checkPoint := chain.GetCheckPoint()
		if checkPoint.Height == 0 {
			secBegin = 0
		} else {
			secBegin = checkPoint.Height/params.BloomBitsSize + 1
		}

		for secId := secBegin; secId <= currentSecId; secId++ {
			// To see if this section has been saved
			saved := dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, secId)
			if saved {
				if block, err := chain.GetMainBranchBlock(secId*params.BloomBitsSize + params.BloomBitsSize - 1); err == nil {
					if b.blockExecutedHead.Header.Height < block.Header.Height {
						b.blockExecutedHead = block
					}
				}
				continue
			}

			// If the entire section has not been saved, then check if each item(a block) in it has been executed. If the block has been executed, put it into the buffer.
			sectionHeadHeight := secId * params.BloomBitsSize
			for i := sectionHeadHeight; i <= currentBlock.Header.Height && i < sectionHeadHeight+params.BloomBitsSize; i++ {
				if block, err := chain.GetMainBranchBlock(i); err == nil && dbaccessor.ReadBlockStateCheck(b.chainDb, block.FullHash()) == types.BlockStateChecked {
					if b.blockExecutedHead.Header.Height < block.Header.Height {
						b.blockExecutedHead = block
					}
					bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, nil)
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

				// TODO: if is fixPoint, clear the later bloom data, and set 'blockExecutedHead'

				if b.blockExecutedHead.Header.Height <= block.Header.Height {
					b.blockExecutedHead = block
				}

				b.insertBloom(block, bloomInfoList)
				continue
			}

			// BlockMined
			if b.blockExecutedHead.SimpleHash() == block.Header.ParentHash {
				// main chain head
				b.blockExecutedHead = block
				log.Info("eventLoop: main chain head grows", "currentHeight", b.blockExecutedHead.Header.Height)

				b.insertBloom(block, bloomInfoList)
			} else if chain.IsReOrg(block, b.blockExecutedHead) {
				// reOrg will not occur when fast sync
				reOrg := dbaccessor.FindReorgChain(b.chainDb, &b.blockExecutedHead.Header, &block.Header)
				if len(reOrg) <= 1 {
					log.Error("eventLoop: reOrg happen. find common ancestor error") // should not happen
					continue                                                         // at least have two element (common ancestor,  and another block which is more preferred than original main chain head)
				}

				commonAncestor := reOrg[0]
				commonAncestorHeight := commonAncestor.Height

				log.Info("eventLoop: reOrg happen", "commonAncestorHeight", commonAncestorHeight, "blockHeight", block.Header.Height)

				b.clearHigherBlooms(commonAncestorHeight+1, b.blockExecutedHead.Header.Height, bloomInfoList, blockChain)

				b.blockExecutedHead = block

				// reOrg[0] is commonAncestor, don't need to insert
				for i := 1; i < len(reOrg)-1; i++ {
					newBlock := blockChain.GetBlock(reOrg[i].FullHash())
					if newBlock != nil {
						b.insertBloom(newBlock, bloomInfoList)
					}
				}

				// set the last block bloom
				b.insertBloom(block, bloomInfoList)
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
			switch res.updateType {
			case build:
				data := res.data.(sectionBuildNotify)
				// Process the newly defined section in the background
				err := b.writer.WriteSection(data.sectionNum, data.sectionBloom)
				if err != nil {
					// If processing failed, don't retry until further notification
					log.Error("Section processing failed", "section", data.sectionNum, "err", err)
				}
			case bloomReplace:
				data := res.data.(sectionBloomReplaceNotify)
				err := b.writer.ReplaceSectionBit(data.sectionNum, data.bloomHeight, data.bloomBit)
				if err != nil {
					log.Error("Can't replace section bit", "section", data.sectionNum, "height", data.bloomHeight, "err", err)
				}
			case clear:
				data := res.data.(sectionClearNotify)
				b.writer.ClearSection(data.sectionNum)

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
				b.update <- sectionUpdateNotify{build, sectionBuildNotify{
					sectionNum:   currentSection - 1,
					sectionBloom: prevSectionBloom.Blooms[:],
				}}
				delete(bloomInfoList.BListMap, currentSection-1)
			}
		}

		if nextSectionBloom, exist := bloomInfoList.BListMap[currentSection+1]; (exist && nextSectionBloom.IsFull()) || dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, currentSection+1) {
			b.update <- sectionUpdateNotify{build, sectionBuildNotify{
				sectionNum:   currentSection,
				sectionBloom: currentSectionBloom.Blooms[:],
			}}
			delete(bloomInfoList.BListMap, currentSection)
		}
	}
}

func (b *BloomIndexer) insertBloom(block *types.Block, bloomInfoList *logbloom.BloomList) {
	blockSectionId := block.Header.Height / params.BloomBitsSize
	if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, blockSectionId) {
		// replace db
		thisBloom := block.Bloom()
		if thisBloom == nil {
			thisBloom, _ = dbaccessor.ReadBloom(b.chainDb, block.FullHash())
		}
		b.update <- sectionUpdateNotify{bloomReplace, sectionBloomReplaceNotify{
			sectionNum:  blockSectionId,
			bloomHeight: block.Header.Height,
			bloomBit:    thisBloom,
		}}
	} else {
		// replace or insert to cache map
		bloomInfoList.Mu.Lock()
		currentSection := bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, block.Bloom())
		b.sendSaveBloomMapChan(bloomInfoList, currentSection)
		bloomInfoList.Mu.Unlock()
	}

	b.bloomInsertFeed.Send(types.BloomInsertEvent{Block: block})
}

// include the input fromHeight
func (b *BloomIndexer) clearHigherBlooms(fromHeight uint64, oldHeadHeight uint64, bloomInfoList *logbloom.BloomList, chain blockChain) {
	bloomInfoList.Mu.Lock()
	defer bloomInfoList.Mu.Unlock()

	sectionId := fromHeight / params.BloomBitsSize

	// 1. clear the later sections
	oldHeadSectionId := oldHeadHeight / params.BloomBitsSize
	for i := sectionId + 1; i <= oldHeadSectionId; i++ {
		if _, exist := bloomInfoList.BListMap[i]; exist {
			log.Info("eventLoop: clear section in map", "section", i)
			delete(bloomInfoList.BListMap, i)
		} else if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, i) {
			log.Info("eventLoop: clear section in db", "section", i)
			b.update <- sectionUpdateNotify{clear, sectionClearNotify{sectionNum: i}}
		}
	}

	// 2. clear the current section
	if dbaccessor.ReadBloomSectionSavedFlag(b.chainDb, sectionId) {
		log.Info("eventLoop: clear current section in db", "section", sectionId)
		b.update <- sectionUpdateNotify{clear, sectionClearNotify{sectionNum: sectionId}}

		for i := sectionId * params.BloomBitsSize; i < fromHeight; i++ {
			block, err := chain.GetMainBranchBlock(i)
			if err != nil {
				log.Error("clearHigherBlooms: can't get main branch block", "height", i, "error", err)
				continue
			}
			bloomInfoList.InsertBlockBloom(b.chainDb, &block.Header, nil)
		}
	} else {
		log.Info("eventLoop: del bloom in map", "section", sectionId)
		for i := fromHeight; i <= oldHeadHeight && i < sectionId*params.BloomBitsSize+params.BloomBitsSize; i++ {
			bloomInfoList.DelBlockBloom(i)
		}
	}
}

func (b *BloomIndexer) SubscribeInsertBloomEvent(ch chan<- types.BloomInsertEvent) event.Subscription {
	return b.bloomInsertFeed.Subscribe(ch)
}
