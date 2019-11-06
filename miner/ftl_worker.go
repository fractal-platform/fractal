// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package miner contains implementations for block mining strategy.
package miner

import (
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/diffculty"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

const (
	// resultQueueSize is the size of channel listening to sealing result.
	resultQueueSize = 10
)

var (
	// maxUint256 is a big integer representing 2^256-1
	maxUint256 = new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
)

type taskItem struct {
	block   *types.Block
	balance uint64
	pubkey  [crypto.BlsPubkeyLen]byte
}

// task contains all information for consensus engine sealing and result submitting.
type task struct {
	items       []*taskItem
	block       *types.Block
	pubkey      [crypto.BlsPubkeyLen]byte
	parentBlock *types.Block
	createdAt   time.Time
}

// worker is the main object which takes care of submitting new work to consensus engine
// and gathering the sealing result.
type worker struct {
	chain       blockChain
	txPool      pool.Pool
	packagePool pool.Pool

	txExecutor txexec.TxExecutor

	// Subscriptions
	chainUpdateCh  chan types.ChainUpdateEvent
	chainUpdateSub event.Subscription

	// Channels
	taskCh   chan *task
	resultCh chan *task
	startCh  chan struct{}
	exitCh   chan struct{}

	wg sync.WaitGroup // for shutdown sync

	newMinedBlockFeed *event.Feed

	mu       sync.RWMutex // The lock used to protect the coinbase and extra fields
	keyman   *keys.MiningKeyManager
	coinbase common.Address

	// atomic status counters
	running int32 // The indicator whether the consensus engine is running or not.
}

func newWorker(chain blockChain, executor txexec.TxExecutor, txPool pool.Pool, pkgPool pool.Pool, newMinedBlockFeed *event.Feed, keyman *keys.MiningKeyManager) *worker {
	worker := &worker{
		chain:             chain,
		txPool:            txPool,
		packagePool:       pkgPool,
		txExecutor:        executor,
		chainUpdateCh:     make(chan types.ChainUpdateEvent),
		taskCh:            make(chan *task),
		resultCh:          make(chan *task, resultQueueSize),
		exitCh:            make(chan struct{}),
		startCh:           make(chan struct{}, 1),
		newMinedBlockFeed: newMinedBlockFeed,
		keyman:            keyman,
	}

	// Subscribe events for blockchain
	worker.chainUpdateSub = worker.chain.SubscribeChainUpdateEvent(worker.chainUpdateCh)

	go worker.mainLoop()
	go worker.resultLoop()
	go worker.taskLoop()

	// Submit first work to initialize pending state.
	worker.startCh <- struct{}{}

	return worker
}

// setCoinbase sets the coinbase used to initialize the block coinbase field.
func (w *worker) setCoinbase(addr common.Address) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.coinbase = addr
}

// start sets the running status as 1 and triggers new work submitting.
func (w *worker) start() {
	atomic.StoreInt32(&w.running, 1)
	w.startCh <- struct{}{}
}

// stop sets the running status as 0.
func (w *worker) stop() {
	atomic.StoreInt32(&w.running, 0)
}

// isRunning returns an indicator whether worker is running or not.
func (w *worker) isRunning() bool {
	return atomic.LoadInt32(&w.running) == 1
}

// close terminates all background threads maintained by the worker and cleans up buffered channels.
// Note the worker does not support being closed multiple times.
func (w *worker) close() {
	atomic.StoreInt32(&w.running, 0)
	close(w.exitCh)
	// Clean up buffered channels
	for empty := false; !empty; {
		select {
		case <-w.resultCh:
		default:
			empty = true
		}
	}

	w.wg.Wait()
	log.Info("miner is stopped")
}

// mainLoop is a standalone goroutine to regenerate the sealing task based on the received event.
func (w *worker) mainLoop() {
	w.wg.Add(1)
	defer w.wg.Done()
	defer w.chainUpdateSub.Unsubscribe()

	for {
		select {
		case <-w.startCh:
			w.commitNewWork()
		case <-w.chainUpdateCh:
			w.commitNewWork()
		case <-w.exitCh:
			return
		case <-w.chainUpdateSub.Err():
			return
		}
	}
}

// seal pushes a sealing task to consensus engine and submits the result.
func (w *worker) seal(t *task, stop <-chan struct{}) {
	var (
		round = uint64(time.Now().UnixNano() / (1e9 / params.RoundsPerSecond))
	)

	ticker := time.NewTicker(time.Millisecond * 10)
search:
	for {
		select {
		case <-stop:
			// Mining terminated, update stats and abort
			log.Debug("Fractal round search aborted")
			break search

		case <-ticker.C:
			if !w.isRunning() {
				break search
			}

			// Compute the PoS value of this round
			currentRoundMills := uint64(time.Now().UnixNano() / 1e6)
			currentRound := uint64(time.Now().UnixNano() / (1e9 / params.RoundsPerSecond))
			if currentRound <= round {
				continue search
			}
			if currentRound-round > 1 {
				log.Error("seal round miss", "miss", currentRound-round)
			}
			round = currentRound

			for _, item := range t.items {
				if currentRound <= item.block.Header.Round {
					continue
				}

				var tryBlock types.Block
				tryBlock.Header.Round = currentRound
				tryBlock.Header.ParentHash = item.block.SimpleHash()

				if !w.chain.GetChainConfig().BlockSigFake {
					var err error
					tryBlock.Header.Sig, err = w.keyman.Sign(w.coinbase, item.pubkey, tryBlock.SignHashByte())
					if err != nil {
						log.Error("seal new block: SignHashWithPassphrase error", "error", err)
						continue search
					}
				}

				// calc digest
				digest := tryBlock.SimpleHash()
				//log.Trace("seal digest", "digest", digest.String())

				// compare
				curDifficulty := difficulty.CalcDifficulty(tryBlock.Header.Round, item.block.Header.Round, item.block.Header.Difficulty)
				target := new(big.Int).Div(new(big.Int).Mul(new(big.Int).SetUint64(item.balance), maxUint256), curDifficulty)
				//log.Info("worker seal","stake",stake,"target",target)
				if new(big.Int).SetBytes(digest.Bytes()).Cmp(target) <= 0 {
					// Correct round found, create a new block with it
					t.block = types.NewBlock(tryBlock.Header.ParentHash, tryBlock.Header.Round,
						tryBlock.Header.Sig, w.coinbase, curDifficulty, item.block.Header.Height+1)
					t.block.ReceivedAt = time.Now()
					t.pubkey = item.pubkey
					t.parentBlock = item.block
					t.block.Header.MinedTime = currentRoundMills
					// Seal and return a block (if still needed)
					select {
					case w.resultCh <- t:
						log.Info("Fractal round found and reported", "round", round, "hash", t.block.SimpleHash(),
							"height", t.block.Header.Height, "parentHash", item.block.FullHash())
					case <-stop:
						log.Debug("Fractal round found but discarded", "round", round)
					}
					break search
				}
			}
		}
	}
	ticker.Stop()
}

// taskLoop is a standalone goroutine to fetch sealing task from the generator and
// push them to consensus engine.
func (w *worker) taskLoop() {
	var (
		stopCh chan struct{}
	)

	// interrupt aborts the in-flight sealing task.
	interrupt := func() {
		log.Debug("miner worker task interrupted")
		if stopCh != nil {
			close(stopCh)
			stopCh = nil
		}
	}
	for {
		select {
		case task := <-w.taskCh:
			interrupt()
			stopCh = make(chan struct{})
			go w.seal(task, stopCh)
		case <-w.exitCh:
			interrupt()
			return
		}
	}
}

// resultLoop is a standalone goroutine to handle sealing result submitting
// and flush relative data to the database.
func (w *worker) resultLoop() {
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case result := <-w.resultCh:
			// Short circuit when receiving empty result.
			if result == nil {
				continue
			}

			block := result.block
			block.ReceivedPath = types.BlockMined
			parentBlock := result.parentBlock

			// check statedb for parent block
			stateOK := w.chain.CalcAndCheckState(parentBlock)
			if !stateOK {
				// if parent state is error, we skip this block
				log.Error("Parent Block State error", "parentHash", parentBlock.FullHash())
				continue
			}

			// fetch state from parent block
			stateDb, err := w.chain.StateAt(parentBlock.Header.StateHash)
			if err != nil {
				log.Error("StateHash for parent block is wrong", "stateHash", parentBlock.Header.StateHash, "err", err.Error())
				continue
			}

			// set ParentFullHash
			block.Header.ParentFullHash = parentBlock.FullHash()
			block.Header.GasLimit = types.CalcGasLimit(parentBlock)
			log.Info("GasLimit adjustment", "parentUsed", parentBlock.Header.GasUsed, "parentLimit", parentBlock.Header.GasLimit, "currentLimit", block.Header.GasLimit)

			var confirmedBlocks types.Blocks
			if block.Header.Height == 0 {
				log.Warn("Genesis block should not be mined")
				return
			} else if block.Header.Height > 1 {
				grandParentBlock := w.chain.GetBlock(parentBlock.Header.ParentFullHash)
				roundRangeBlocks := w.chain.GetBlocksFromBlockRange(grandParentBlock, parentBlock)
				log.Info("Get blocks from round1 to round2", "simpleHash", block.SimpleHash(), "round1", grandParentBlock.Header.Round,
					"round2", parentBlock.Header.Round, "numbers", len(roundRangeBlocks), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

				// use a set to kick out duplicated simple hash
				confirmedBlockSimpleHashSet := mapset.NewSet()

				for _, confirmBlk := range roundRangeBlocks {
					// kick out parent block
					if confirmBlk.FullHash() == parentBlock.FullHash() {
						continue
					}

					// kick out duplicated simple hash
					if confirmedBlockSimpleHashSet.Contains(confirmBlk.SimpleHash()) {
						log.Warn("Same simpleHash but different fullHash", "simpleHash", confirmBlk.SimpleHash(), "fullHash", confirmBlk.FullHash())
						continue
					}

					if check, _ := w.chain.CheckGreedy(confirmBlk, parentBlock, uint64(w.chain.GetGreedy())); check {
						block.Header.Confirms = append(block.Header.Confirms, confirmBlk.FullHash())
						confirmedBlocks = append(confirmedBlocks, confirmBlk)
						confirmedBlockSimpleHashSet.Add(confirmBlk.SimpleHash())
					}
				}
			}

			prevStateDb, _, _ := w.chain.GetStateBeforeCacheHeight(parentBlock, uint8(params.ConfirmHeightDistance-1))

			// pre pack
			block.Body.TxPackageHashes = w.packTxPackages(block, prevStateDb, stateDb)
			block.Body.Transactions = w.packTransactions(block, prevStateDb, stateDb)
			block.Header.TxHash = types.DeriveSha(types.Transactions(block.Body.Transactions))
			log.Info("finish packing packages and transactions", "simpleHash", block.SimpleHash(), "packages", len(block.Body.TxPackageHashes),
				"transactions", len(block.Body.Transactions), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

			// exec tx
			var (
				receipts    types.Receipts
				executedTxs []*types.TxWithIndex
				allLogs     []*types.Log
				usedGas     = new(uint64)
				gasPool     = new(types.GasPool).AddGas(block.Header.GasLimit)
				txpkgs      = w.chain.GetTxPackageList(block.Body.TxPackageHashes)

				pkgExecLoopEndIndex int
				txExecLoopEndIndex  int
			)

			callbackParamKey := wasm.GetGlobalRegisterParam().RegisterParam(stateDb, block)
			executedTxs, allLogs, receipts, pkgExecLoopEndIndex = w.txExecutor.ExecuteTxPackages(txpkgs, prevStateDb, stateDb, receipts, block, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)
			executedTxs, _, receipts, txExecLoopEndIndex = w.txExecutor.ExecuteTransactions(block.Body.Transactions, prevStateDb, stateDb, receipts, block, types.NotInPackage, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)
			wasm.GetGlobalRegisterParam().UnRegisterParam(callbackParamKey)
			log.Info("finish executing packages and transactions", "simpleHash", block.SimpleHash(), "executedTxs", len(executedTxs), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

			// final pack
			block.Body.TxPackageHashes = block.Body.TxPackageHashes[0:pkgExecLoopEndIndex]
			block.Body.Transactions = block.Body.Transactions[0:txExecLoopEndIndex]

			block.Header.TxHash = types.DeriveSha(types.Transactions(block.Body.Transactions))

			block.Header.GasUsed = *usedGas

			// set reward
			state.AddBlockReward(stateDb, block, confirmedBlocks)

			block.Header.StateHash = stateDb.IntermediateRoot(true)

			//
			block.Header.Amount = parentBlock.Header.Amount + uint64(len(block.Header.Confirms)) + 1

			var bloom *types.Bloom
			if len(receipts) == 0 {
				block.Header.ReceiptHash = types.DeriveSha(types.Receipts{})
				bloom = &types.Bloom{}
			} else {
				block.Header.ReceiptHash = types.DeriveSha(types.Receipts(receipts))
				bloom = types.CreateBloom(receipts)
			}
			block.CacheBloom(bloom)

			if !w.chain.GetChainConfig().BlockSigFake {
				FullSig, err := w.keyman.Sign(w.coinbase, result.pubkey, block.FullHash().Bytes())
				if err != nil {
					log.Error("calc full sig: SignHashWithPassphrase error", "error", err)
					continue
				}
				block.Header.FullSig = FullSig
			}
			log.Info("finish signing block", "simpleHash", block.SimpleHash(), "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

			// Short circuit when receiving duplicate result caused by resubmitting.
			if w.chain.HasBlock(block.FullHash()) {
				continue
			}

			// Commit block and state to database.
			w.chain.InsertBlockWithState(block, stateDb, receipts, executedTxs, bloom)

			// broadcast new mined block
			w.newMinedBlockFeed.Send(types.NewMinedBlockEvent{Block: block})
			log.Info("Mined a new block", "height", block.Header.Height, "round", block.Header.Round, "hash", block.FullHash())

		case <-w.exitCh:
			return
		}
	}
}

// commitNewWork generates several new sealing tasks based on the parent block.
func (w *worker) commitNewWork() {
	w.mu.RLock()
	defer w.mu.RUnlock()

	if w.coinbase.Big().Cmp(common.Big0) == 0 {
		keys := w.keyman.Keys()
		for addr := range keys {
			log.Info("Set coinbase", "coinbase", addr)
			w.coinbase = addr
			break
		}
	}

	var items []*taskItem
	greedy := w.chain.GetGreedy()
	parentBlocks := w.chain.GetGreedyBlocks(greedy)
	for _, block := range parentBlocks {
		balance, pubkey, err := w.chain.GetPreBalanceAndPubkey(block, w.coinbase)
		if err != nil {
			log.Error("Get balance and pubkey failed", "parentBlock", block.FullHash(), "coinbase", w.coinbase)
			continue
		}
		//log.Info("new task", "block", block.FullHash(), "pubkey", hexutil.Encode(pubkey[:]), "balance", balance, "coinbase", w.coinbase)
		if balance == 0 || pubkey == nil || len(pubkey) != crypto.BlsPubkeyLen {
			continue
		} else {
			item := &taskItem{
				block:   block,
				balance: balance,
			}
			copy(item.pubkey[:], pubkey)
			items = append(items, item)
		}
	}

	if w.isRunning() {
		select {
		case w.taskCh <- &task{items: items, createdAt: time.Now()}:
			log.Info("Commit new mining work", "greedy", greedy, "parentBlockSize", len(items))

		case <-w.exitCh:
			log.Info("Worker has exited")
		}
	}
}

func (w *worker) packTxPackages(block *types.Block, prevStateDb *state.StateDB, stateDb *state.StateDB) []common.Hash {
	// Fill the block with all available pending packages.
	queue := w.packagePool.ContentForPack()
	// Short circuit if there is no available pending packages
	if len(queue) == 0 {
		return nil
	}

	return w.flattenPackages(queue, block, prevStateDb, stateDb)
}

func (w *worker) flattenPackages(queue map[common.Address][]pool.Element, block *types.Block, prevStateDb *state.StateDB, stateDb *state.StateDB) []common.Hash {
	var packedTxPackageHashes []common.Hash

	// Short circuit if current is nil
	for addr, txPkgs := range queue {
		currentNonceSet := stateDb.PackageNonceSet(addr)
		if currentNonceSet == nil {
			// nonceSet is nil, the addr has no state, so we don't handle for this packer
			continue
		}

		nonceSet := nonces.NewNonceSet(currentNonceSet) // make a new nonceSet to guarantee readonly (used here only for queries)
		if prevStateDb != nil {
			nonceSet.Reset(prevStateDb.GetPackageNonce(addr))
		}
		for _, txPkg := range txPkgs {
			pkg := txPkg.(*types.TxPackage)
			pkgHash := txPkg.Hash()
			searchResult := nonceSet.Search(txPkg.Nonce(), w.chain.GetChainConfig().MaxNonceBitLength)
			if searchResult != nonces.NotContainedAndAllowed {
				if searchResult != nonces.Contained {
					log.Info("ignore package", "hash", pkgHash, "nonce", txPkg.Nonce(), "addr", addr, "searchResult", searchResult)
				}
				continue
			}

			// check the greedy rules for pkg related block
			relateBlockHash := pkg.BlockFullHash()
			relateBlock := w.chain.GetBlock(relateBlockHash)
			if relateBlock == nil {
				log.Warn("ignore package", "blockHash", relateBlockHash, "pkgHash", pkgHash, "err", "related block is not exist")
			}
			parentBlock := w.chain.GetBlock(block.Header.ParentFullHash)
			check, err := w.chain.CheckGreedy(relateBlock, parentBlock, uint64(w.chain.GetGreedy()))
			if err != nil {
				log.Warn("ignore package(greedy check error)", "blockHash", relateBlockHash, "pkgHash", pkgHash, "err", err)
				continue
			}
			if !check {
				log.Warn("ignore package(greedy check failed)", "blockHash", relateBlockHash, "pkgHash", pkgHash)
				continue
			}

			if err := w.chain.ValidatePackage(pkg, block.Header.Height); err != nil {
				log.Info("ignore package(validate package failed)", "blockHash", pkg.BlockFullHash(), "pkgHash", pkgHash, "err", err)
				continue
			}

			packedTxPackageHashes = append(packedTxPackageHashes, pkgHash)
		}
		log.Debug("flattenPackages: nonceSet info", "start", nonceSet.Start, "bitMask", hexutil.Encode(nonceSet.BitMask), "length", nonceSet.Length)
	}

	log.Info("flattenPackages: pkg packed", "num", len(packedTxPackageHashes))

	return packedTxPackageHashes
}

func (w *worker) packTransactions(block *types.Block, prevStateDb *state.StateDB, stateDb *state.StateDB) types.Transactions {
	// Fill the block with all available pending transactions.
	queue := w.txPool.ContentForPack()
	// Short circuit if there is no available pending transactions
	if len(queue) == 0 {
		return nil
	}

	return w.flattenTransactionsByPrice(block, queue, prevStateDb, stateDb)
}

func (w *worker) flattenTransactionsByPrice(block *types.Block, queue map[common.Address][]pool.Element, prevStateDb *state.StateDB, stateDb *state.StateDB) types.Transactions {
	var packedTxs types.Transactions

	txByPriceAndNonce := pool.NewElementsByPriceAndNonce(queue)

	getTxNonceSetAndBalance := func() func(address common.Address) (*nonces.NonceSet, *big.Int) {
		var nonceSetCache = make(map[common.Address]*nonces.NonceSet)
		var balanceCache = make(map[common.Address]*big.Int)
		return func(address common.Address) (*nonces.NonceSet, *big.Int) {
			nonceSet, ok := nonceSetCache[address]
			if !ok {
				currentNonceSet := stateDb.TxNonceSet(address)
				if currentNonceSet == nil {
					nonceSet = nil
				} else {
					nonceSet = nonces.NewNonceSet(currentNonceSet) // make a new nonceSet to guarantee readonly (used here only for queries)
					if prevStateDb != nil {
						nonceSet.Reset(prevStateDb.GetNonce(address))
					}
				}
				nonceSetCache[address] = nonceSet
			}
			balance, ok := balanceCache[address]
			if !ok {
				balance = stateDb.GetBalance(address)
				balanceCache[address] = balance
			}
			return nonceSet, balance
		}
	}()

	for {
		ele := txByPriceAndNonce.Peek()
		if ele.Element == nil {
			break
		}
		txByPriceAndNonce.Shift()

		from := ele.From
		tx := ele.Element.(*types.Transaction)

		nonceSet, balance := getTxNonceSetAndBalance(from)
		if nonceSet == nil {
			// nonceSet is nil, the addr has no state, so we don't pack transaction for him
			continue
		}

		cost := tx.Cost()
		if cost.Cmp(balance) > 0 {
			log.Info("ignore tx: insufficient balance", "address", from, "hash", tx.Hash(), "balance", balance.Uint64(), "cost", cost.Uint64())
			continue
		}

		if tx.Gas() > block.Header.GasLimit {
			log.Info("ignore tx: gas limit error", "address", from, "hash", tx.Hash(), "txGasLimit", tx.Gas(), "blockGasLimit", block.Header.GasLimit)
			continue
		}

		searchResult := nonceSet.Search(tx.Nonce(), w.chain.GetChainConfig().MaxNonceBitLength)
		if searchResult != nonces.NotContainedAndAllowed {
			log.Info("ignore tx: nonce error", "nonce", tx.Nonce(), "addr", from, "searchResult", searchResult)
			continue
		}
		packedTxs = append(packedTxs, tx)
	}

	log.Debug("flattenTransactions: tx packed", "num", len(packedTxs))

	return packedTxs
}
