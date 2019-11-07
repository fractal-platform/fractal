package chain

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/math"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

type blockWithExecutedTx struct {
	block       *types.Block
	executedTxs []*types.TxWithIndex
}

type blockWithExecutedTxHeap []*blockWithExecutedTx

func (s blockWithExecutedTxHeap) Len() int           { return len(s) }
func (s blockWithExecutedTxHeap) Less(i, j int) bool { return s[i].block.Header.Height < s[j].block.Header.Height }
func (s blockWithExecutedTxHeap) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

func (s *blockWithExecutedTxHeap) Push(x interface{}) {
	*s = append(*s, x.(*blockWithExecutedTx))
}

func (s *blockWithExecutedTxHeap) Pop() interface{} {
	old := *s
	n := len(old)
	x := old[n-1]
	*s = old[0 : n-1]
	return x
}

type TxInChainProcessor struct {
	chain *BlockChain

	blockHeap blockWithExecutedTxHeap
	heapMu    sync.RWMutex

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup // for shutdown sync
}

func NewTxInChainProcessor(chain *BlockChain, processPeriod int) *TxInChainProcessor {
	log.Info("NewTxInChainProcessor: Init TxInChainProcessor")
	var p = &TxInChainProcessor{
		chain:     chain,
		blockHeap: make(blockWithExecutedTxHeap, 0),
	}

	// init blockHeap
	savedHeight, savedHash, err := dbaccessor.ReadTxSavedBlockHeightAndHash(p.chain.db)
	if err == nil {
		// has history data:

		// 1. make sure the history data is right
		if mainBlock, err := p.chain.GetMainBranchBlock(savedHeight); err == nil {
			if mainBlock.FullHash() != savedHash {
				// wrong history data
				log.Info("NewTxInChainProcessor: wrong history data, need to recover")
				old := p.chain.GetBlock(savedHash)
				chain := dbaccessor.FindReorgChain(p.chain.db, &old.Header, &mainBlock.Header)
				if len(chain) <= 1 {
					log.Error("NewTxInChainProcessor: unexpected error, recorg chain length < 1")
					panic("NewTxInChainProcessor: unexpected error, recorg chain length < 1")
				}
				for i := 1; i < len(chain); i++ {
					hash := chain[i].FullHash()
					block := p.chain.GetBlock(hash)
					var confirmBlocks types.Blocks
					for _, fullHash := range block.Header.Confirms {
						var confirmBlock = p.chain.GetBlock(fullHash)
						confirmBlocks = append(confirmBlocks, confirmBlock)
					}
					_, _, executedTxs, _, err := p.chain.execBlock(block, confirmBlocks)
					if err == nil {
						dbaccessor.WriteTxLookupEntries(p.chain.db, block.Header.Height, hash, executedTxs)
					} else {
						panic("NewTxInChainProcessor: unexpected error, execBlock failed1")
					}
				}
				dbaccessor.WriteTxSavedBlockHeightAndHash(p.chain.db, mainBlock.Header.Height, mainBlock.FullHash())
			}
		}

		// 2. save data (savedHeight, current] to db
		access := p.chain.CurrentBlock()
		for access.Header.Height > savedHeight {
			var confirmBlocks types.Blocks
			for _, fullHash := range access.Header.Confirms {
				var confirmBlock = p.chain.GetBlock(fullHash)
				confirmBlocks = append(confirmBlocks, confirmBlock)
			}
			_, _, executedTxs, _, err := p.chain.execBlock(access, confirmBlocks)
			if err == nil {
				dbaccessor.WriteTxLookupEntries(p.chain.db, access.Header.Height, access.FullHash(), executedTxs)
			} else {
				panic("NewTxInChainProcessor: unexpected error, execBlock failed2")
			}

			// previous
			access = p.chain.GetBlock(access.Header.ParentFullHash)
			if access == nil {
				panic("NewTxInChainProcessor: unexpected error, cannot find block")
			}
		}
		dbaccessor.WriteTxSavedBlockHeightAndHash(p.chain.db, p.chain.CurrentBlock().Header.Height, p.chain.CurrentBlock().FullHash())
	}

	p.ctx, p.cancel = context.WithCancel(context.Background())
	go p.loop(time.NewTicker(time.Duration(processPeriod) * time.Second))
	return p
}

func (p *TxInChainProcessor) AddBlock(b *blockWithExecutedTx) {
	if b.block.ReceivedPath == types.BlockFastSync {
		// save to db directly
		dbaccessor.WriteTxLookupEntries(p.chain.db, b.block.Header.Height, b.block.FullHash(), b.executedTxs)
		savedHeight, _, err := dbaccessor.ReadTxSavedBlockHeightAndHash(p.chain.db)
		if err != nil || b.block.Header.Height > savedHeight {
			// save
			dbaccessor.WriteTxSavedBlockHeightAndHash(p.chain.db, b.block.Header.Height, b.block.FullHash())
		}
		return
	}

	p.heapMu.Lock()
	log.Info("TxInChainProcessor: addblock", "height", b.block.Header.Height, "hash", b.block.FullHash())
	heap.Push(&p.blockHeap, b)
	p.heapMu.Unlock()
}

func (p *TxInChainProcessor) SearchTransactionInHeap(txHash common.Hash) (*types.Transaction, common.Hash) {
	p.heapMu.RLock()
	defer p.heapMu.RUnlock()

	for _, value := range p.blockHeap {
		if !p.chain.IsInMainBranch(value.block) {
			continue
		}

		for _, tx := range value.executedTxs {
			if tx.Tx.Hash() == txHash {
				return tx.Tx, value.block.FullHash()
			}
		}
	}

	return nil, common.Hash{}
}

func (p *TxInChainProcessor) loop(ticker *time.Ticker) {
	p.wg.Add(1)
	defer p.wg.Done()

	for {
		select {
		case <-ticker.C:
			currentHeight := p.chain.CurrentBlock().Header.Height

			// 1. health check. if reorg happened, reset the height <= txSavedHeight
			var savedHeight uint64
			var savedHash common.Hash
			var err error
			if savedHeight, savedHash, err = dbaccessor.ReadTxSavedBlockHeightAndHash(p.chain.db); err == nil {
				if mainBlock, err := p.chain.GetMainBranchBlock(savedHeight); err == nil {
					if mainBlock.FullHash() != savedHash {
						log.Info("TxInChainProcessor loop: reorg happe", "savedHeight", savedHeight)
						old := p.chain.GetBlock(savedHash)
						chain := dbaccessor.FindReorgChain(p.chain.db, &old.Header, &mainBlock.Header)
						if len(chain) <= 1 {
							panic("recorg chain length < 1")
						}
						for i := 1; i < len(chain); i++ {
							hash := chain[i].FullHash()
							block := p.chain.GetBlock(hash)
							var confirmBlocks types.Blocks
							for _, fullHash := range block.Header.Confirms {
								var confirmBlock = p.chain.GetBlock(fullHash)
								confirmBlocks = append(confirmBlocks, confirmBlock)
							}
							_, _, executedTxs, _, err := p.chain.execBlock(block, confirmBlocks)
							if err == nil {
								dbaccessor.WriteTxLookupEntries(p.chain.db, block.Header.Height, hash, executedTxs)
							} else {
								panic("TxInChainProcessor loop: unexpected error, execBlock failed")
							}
						}
						dbaccessor.WriteTxSavedBlockHeightAndHash(p.chain.db, mainBlock.Header.Height, mainBlock.FullHash())
					}
				}
			}

			// 2. process heap
			var stableHeight uint64
			if currentHeight < params.ConfirmHeightDistance {
				continue
			}
			stableHeight = currentHeight - params.ConfirmHeightDistance

			var processedHeight uint64 = math.MaxUint64
			for {
				p.heapMu.Lock()
				if p.blockHeap.Len() == 0 {
					p.heapMu.Unlock()
					break
				}

				accessBlock := p.blockHeap[0].block
				accessTxs := p.blockHeap[0].executedTxs

				if accessBlock.Header.Height == processedHeight {
					// this height has been processed
					heap.Pop(&p.blockHeap)
					p.heapMu.Unlock()
					continue
				}

				if accessBlock.Header.Height >= stableHeight {
					p.heapMu.Unlock()
					break
				}

				mainBlockInHeight, err := p.chain.GetMainBranchBlock(accessBlock.Header.Height)
				if err != nil {
					p.heapMu.Unlock()
					break
				}

				heap.Pop(&p.blockHeap)
				p.heapMu.Unlock()

				if mainBlockInHeight.FullHash() == accessBlock.FullHash() {
					processedHeight = accessBlock.Header.Height
					// save to db
					dbaccessor.WriteTxLookupEntries(p.chain.db, accessBlock.Header.Height, accessBlock.FullHash(), accessTxs)
					if accessBlock.Header.Height > savedHeight {
						// save
						dbaccessor.WriteTxSavedBlockHeightAndHash(p.chain.db, accessBlock.Header.Height, accessBlock.FullHash())
					}
				}
			}
		case <-p.ctx.Done():
			return
		}
	}
}

func (p *TxInChainProcessor) Stop() {
	if p.cancel != nil {
		p.cancel()
		p.wg.Wait()
		log.Info("tx in chain processor is stopped")
	}
}
