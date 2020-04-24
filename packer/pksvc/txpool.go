package pksvc

import (
	"container/heap"
	"errors"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/transaction"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	ErrTxAlreadyExist            = errors.New("the tx already exists in the pool of packer")
	ErrPoolNotBigEnough          = errors.New("the tx pool doesn't have enough tx")
	ErrTransactionNotMatchPacker = errors.New("the transaction and the packer don't match")
	ErrIsBroadcastTx             = errors.New("the transaction should be broadcast")
)

type indexQueue struct {
	txSigner types.Signer

	queue types.TxByPrice
	index map[common.Hash]*types.Transaction

	mu sync.RWMutex
}

func newIndexQueue(txSigner types.Signer) indexQueue {
	return indexQueue{
		txSigner: txSigner,
		queue:    make(types.TxByPrice, 0),
		index:    make(map[common.Hash]*types.Transaction),
	}
}

func (q *indexQueue) push(tx *types.Transaction) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.pushUnsafe(tx)
}

func (q *indexQueue) pushUnsafe(tx *types.Transaction) error {
	if old, ok := q.index[tx.PackingHash(q.txSigner)]; ok {
		if old.GasPrice().Cmp(tx.GasPrice()) < 0 {
			log.Info("pksvc tx pool: replace a tx", "old price", old.GasPrice().Uint64(), "new price", tx.GasPrice().Uint64())
			*old = *tx
			return nil
		}
		return ErrTxAlreadyExist
	}

	heap.Push(&q.queue, tx)
	q.index[tx.PackingHash(q.txSigner)] = tx
	return nil
}

//
//func (q *indexQueue) pop() *types.Transaction {
//	q.mu.Lock()
//	defer q.mu.Unlock()
//
//	return q.popUnsafe()
//}

func (q *indexQueue) popN(n int) ([]*types.Transaction, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queue.Len() < n {
		return nil, ErrPoolNotBigEnough
	}
	txs := make([]*types.Transaction, n)
	for i := 0; i < n; i++ {
		txs[i] = q.popUnsafe()
	}
	return txs, nil
}

func (q *indexQueue) popUnsafe() *types.Transaction {
	if len(q.queue) == 0 {
		return nil
	}

	t := heap.Pop(&q.queue).(*types.Transaction)
	delete(q.index, t.PackingHash(q.txSigner))
	return t
}

func (q *indexQueue) len() int {
	return q.queue.Len()
}

type txPool struct {
	packerIndex     *uint32
	queue           indexQueue
	fakeMode        bool
	signer          types.Signer
	chain           blockChain
	packerGroupSize uint64

	newTxCh chan *types.Transaction
	mu      sync.RWMutex
}

func newTxPool(fakeMode bool, newTxCh chan *types.Transaction, signer types.Signer, chain blockChain, packerGroupSize uint64) *txPool {
	return &txPool{
		queue:           newIndexQueue(signer),
		fakeMode:        fakeMode,
		signer:          signer,
		newTxCh:         newTxCh,
		chain:           chain,
		packerGroupSize: packerGroupSize,
	}
}

func (pool *txPool) SetPackerIndex(index uint32) {
	pool.packerIndex = &index
}

func (pool *txPool) Add(tx *types.Transaction) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()
	return pool.add(tx)
}

func (pool *txPool) AddAll(txs types.Transactions) []error {
	errs := make([]error, len(txs))
	pool.mu.Lock()
	defer pool.mu.Unlock()
	for i, tx := range txs {
		errs[i] = pool.add(tx)
	}
	return errs
}

func (pool *txPool) Count() int {
	return pool.queue.len()
}

func (pool *txPool) Pop(n int) ([]*types.Transaction, error) {
	return pool.queue.popN(n)
}

func (pool *txPool) validate(tx *types.Transaction) error {
	// ignore transaction validate in fake mode
	if pool.fakeMode {
		return nil
	}

	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > 32*1024 {
		return transaction.ErrOversizedData
	}

	if tx.Broadcast() {
		return ErrIsBroadcastTx
	}

	// Whether the transaction and the packer match
	if !tx.MatchPacker(pool.packerGroupSize, *pool.packerIndex, pool.signer) {
		return ErrTransactionNotMatchPacker
	}

	// Transactions can't be negative.
	if tx.Value().Sign() < 0 {
		return transaction.ErrNegativeValue
	}

	// Make sure the transaction is signed properly
	from, err := types.Sender(pool.signer, tx)
	if err != nil {
		return transaction.ErrInvalidSender
	}

	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	currentState, err := pool.currentStateDb()
	if err != nil {
		return err
	}
	
	currentBlock := pool.chain.CurrentBlock()
	if currentState.GetTradableBalance(from, currentBlock.Header.Round).Cmp(tx.Cost()) < 0 {
		return transaction.ErrInsufficientFunds
	}

	intrGas, err := txexec.IntrinsicGas(tx.Data(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return transaction.ErrIntrinsicGas
	}

	if currentBlock.Header.GasLimit < tx.Gas() {
		return transaction.ErrGasLimit
	}

	log.Debug("Validate Tx Ok (when add to tx pool of packer)")

	return nil
}

func (pool *txPool) add(tx *types.Transaction) error {
	if err := pool.validate(tx); err != nil {
		log.Error("validate transaction failed", "err", err)
		return err
	}
	if err := pool.queue.push(tx); err != nil {
		return err
	}
	if pool.newTxCh != nil {
		go func() { pool.newTxCh <- tx }()
	}
	return nil
}

func (pool *txPool) currentStateDb() (*state.StateDB, error) {
	block := pool.chain.CurrentBlock()
	if block == nil {
		return nil, errors.New("block not found")
	}

	stateDb, err := pool.chain.StateAt(block.Header.StateHash)
	if err != nil {
		return nil, err
	}

	return stateDb, nil
}
