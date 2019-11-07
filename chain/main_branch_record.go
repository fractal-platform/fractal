package chain

import (
	"context"
	"errors"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/utils/log"
)

var errGetBlockNil = errors.New("GetBlock error, cannot find the block")

type ChainBackend interface {
	GetBlock(hash common.Hash) *types.Block
	Database() dbwrapper.Database
	CurrentBlock() *types.Block
	SubscribeChainUpdateEvent(ch chan<- types.ChainUpdateEvent) event.Subscription
	SubscribeBlockExecutedEvent(ch chan<- types.BlockExecutedEvent) event.Subscription
}

type MainBranchRecord struct {
	blockChain ChainBackend

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // for shutdown sync
}

func NewMainBranchRecord(bc ChainBackend) *MainBranchRecord {
	m := &MainBranchRecord{
		blockChain: bc,
	}

	_, _, err := dbaccessor.ReadMainBranchHeadHeightAndHash(bc.Database())
	if err != nil {
		genesisBlock := bc.CurrentBlock()
		dbaccessor.WriteHeightBlockMap(bc.Database(), 0, genesisBlock.FullHash())
		dbaccessor.WriteMainBranchHeadHeightAndHash(bc.Database(), 0, genesisBlock.FullHash())
	}

	return m
}

func (m *MainBranchRecord) Start() {
	events := make(chan types.BlockExecutedEvent, 10)
	sub := m.blockChain.SubscribeBlockExecutedEvent(events)
	m.ctx, m.cancel = context.WithCancel(context.Background())

	go m.eventLoop(events, sub)
}

func (m *MainBranchRecord) Stop() {
	if m.cancel != nil {
		m.cancel()
		m.wg.Wait()
		log.Info("main branch record is stopped")
	}
}

func (m *MainBranchRecord) eventLoop(events chan types.BlockExecutedEvent, sub event.Subscription) {
	m.wg.Add(1)
	defer m.wg.Done()
	defer sub.Unsubscribe()

	for {
		select {
		case ev, ok := <-events:
			if !ok {
				log.Error("MainBranchRecord eventLoop: channel error")
				return
			}

			block := ev.Block
			blockReceivedPath := block.ReceivedPath
			log.Info("MainBranchRecord recv block", "block", block.FullHash(), "blockReceivedPath", blockReceivedPath)

			db := m.blockChain.Database()

			savedHeight, hash, err := dbaccessor.ReadMainBranchHeadHeightAndHash(db)
			if err != nil {
				log.Error("MainBranchRecord eventLoop: get main branch height error", "error", err)
				return
			}
			currentHead := m.blockChain.GetBlock(hash)
			if currentHead == nil {
				log.Error("MainBranchRecord eventLoop: get main branch block error", "error", errGetBlockNil)
				return
			}

			// fast sync block
			if blockReceivedPath == types.BlockFastSync {
				log.Info("MainBranchRecord eventLoop: get fast sync block", "height", block.Header.Height)
				// fast sync: store to db directly
				batch := db.NewBatch()
				dbaccessor.WriteHeightBlockMap(batch, block.Header.Height, block.FullHash())
				if savedHeight <= block.Header.Height {
					dbaccessor.WriteMainBranchHeadHeightAndHash(batch, block.Header.Height, block.FullHash())
				}
				batch.Write()
				continue
			}

			// mined block
			if currentHead.SimpleHash() == block.Header.ParentHash {
				log.Info("MainBranchRecord eventLoop: main chain head grows", "height", block.Header.Height)
				batch := db.NewBatch()
				dbaccessor.WriteHeightBlockMap(batch, block.Header.Height, block.FullHash())
				dbaccessor.WriteMainBranchHeadHeightAndHash(batch, block.Header.Height, block.FullHash())
				batch.Write()
			} else if IsReOrg(block, currentHead) {
				log.Info("MainBranchRecord eventLoop: isReOrg", "oldHeight", currentHead.Header.Height, "newHeight", block.Header.Height)
				reorg := dbaccessor.FindReorgChain(db, &currentHead.Header, &block.Header)
				if len(reorg) <= 1 {
					log.Error("MainBranchRecord eventLoop: reorg happen. get common ancestor error") // should not happen
					return
				}
				batch := db.NewBatch()
				for i := 1; i < len(reorg); i++ {
					dbaccessor.WriteHeightBlockMap(batch, reorg[i].Height, reorg[i].FullHash())
				}
				dbaccessor.WriteMainBranchHeadHeightAndHash(batch, block.Header.Height, block.FullHash())
				batch.Write()
			}
		case <-m.ctx.Done():
			return
		}
	}
}

func (m *MainBranchRecord) GetMainBranchBlock(height uint64) (*types.Block, error) {
	hash, err := dbaccessor.ReadHeightBlockMap(m.blockChain.Database(), height)
	if err != nil {
		return nil, err
	}
	block := m.blockChain.GetBlock(hash)
	if block == nil {
		return nil, errGetBlockNil
	}

	return block, nil
}
