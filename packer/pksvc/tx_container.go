package pksvc

import (
	"container/heap"
	"errors"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	ErrTxAlreadyExist            = errors.New("the tx already exists in the tx container of packer")
	ErrContainerNotBigEnough     = errors.New("the tx container doesn't have enough tx")
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
			log.Info("pksvc tx container: replace a tx", "old price", old.GasPrice().Uint64(), "new price", tx.GasPrice().Uint64())
			*old = *tx
			return nil
		}
		return ErrTxAlreadyExist
	}

	heap.Push(&q.queue, tx)
	q.index[tx.PackingHash(q.txSigner)] = tx
	return nil
}

func (q *indexQueue) popN(n int) ([]*types.Transaction, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.queue.Len() < n {
		return nil, ErrContainerNotBigEnough
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
	q.mu.Lock()
	defer q.mu.Unlock()

	return q.queue.Len()
}

type txContainer struct {
	packerIndex     *uint32
	queue           indexQueue
	fakeMode        bool
	signer          types.Signer
	chain           blockChain
	packerGroupSize uint64

	newTxCh chan *types.Transaction
	mu      sync.RWMutex
}

func newTxContainer(fakeMode bool, newTxCh chan *types.Transaction, signer types.Signer, chain blockChain, packerGroupSize uint64) *txContainer {
	return &txContainer{
		queue:           newIndexQueue(signer),
		fakeMode:        fakeMode,
		signer:          signer,
		newTxCh:         newTxCh,
		chain:           chain,
		packerGroupSize: packerGroupSize,
	}
}

func (t *txContainer) SetPackerIndex(index uint32) {
	t.packerIndex = &index
}

func (t *txContainer) Add(tx *types.Transaction) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.add(tx)
}

func (t *txContainer) AddAll(txs types.Transactions) []error {
	errs := make([]error, len(txs))
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, tx := range txs {
		errs[i] = t.add(tx)
	}
	return errs
}

func (t *txContainer) Count() int {
	return t.queue.len()
}

func (t *txContainer) Pop(n int) ([]*types.Transaction, error) {
	return t.queue.popN(n)
}

func (t *txContainer) validate(tx *types.Transaction) error {
	// ignore transaction validate in fake mode
	if t.fakeMode {
		return nil
	}

	// Heuristic limit, reject transactions over 32KB to prevent DOS attacks
	if tx.Size() > 32*1024 {
		return pool.ErrOversizedData
	}

	if tx.Broadcast() {
		return ErrIsBroadcastTx
	}

	// Whether the transaction and the packer match
	if !tx.MatchPacker(t.packerGroupSize, *t.packerIndex, t.signer) {
		return ErrTransactionNotMatchPacker
	}

	// Transactions can't be negative.
	if tx.Value().Sign() < 0 {
		return pool.ErrNegativeValue
	}

	// Make sure the transaction is signed properly
	from, err := types.Sender(t.signer, tx)
	if err != nil {
		return pool.ErrInvalidSender
	}

	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL / 10
	currentState, err := t.currentStateDb()
	if err != nil {
		return err
	}
	if currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return pool.ErrInsufficientFunds
	}

	intrGas, err := txexec.IntrinsicGas(tx.Data(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return pool.ErrIntrinsicGas
	}

	if t.chain.CurrentBlock().Header.GasLimit < tx.Gas() {
		return pool.ErrGasLimit
	}

	log.Debug("Validate Tx Ok (when add to tx container of packer)")

	return nil
}

func (t *txContainer) add(tx *types.Transaction) error {
	if err := t.validate(tx); err != nil {
		log.Error("validate transaction failed", "err", err)
		return err
	}
	if err := t.queue.push(tx); err != nil {
		return err
	}
	if t.newTxCh != nil {
		go func() { t.newTxCh <- tx }()
	}
	return nil
}

func (t *txContainer) currentStateDb() (*state.StateDB, error) {
	block := t.chain.CurrentBlock()
	if block == nil {
		return nil, errors.New("block not found")
	}

	stateDb, err := t.chain.StateAt(block.Header.StateHash)
	if err != nil {
		return nil, err
	}

	return stateDb, nil
}
