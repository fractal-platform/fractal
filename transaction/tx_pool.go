package transaction

import (
	"errors"
	"reflect"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

var TransactionType = reflect.TypeOf(types.Transaction{})

var (
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas limit * price + value")
)

type TxHelper struct {
	signer types.Signer
}

func (h *TxHelper) Reset(p pool.Pool, newHead *types.Block) {
	// Traversing the transactions in the block, comparing the nonce
	for _, newTx := range newHead.Body.Transactions {
		address, _ := types.Sender(h.signer, newTx)
		if newTx.Nonce() >= p.StateUnsafe().GetNonce(address) {
			// Put into the p, this happens only for non-local blocks
			p.AddUnsafe([]pool.Element{newTx}, false)
		}
	}
}

func (h *TxHelper) Validate(p pool.Pool, ele pool.Element, currentState pool.StateDB, blockChain pool.BlockChain) error {
	tx := (ele).(*types.Transaction)

	if !tx.Broadcast() {
		return pool.ErrIsNotAPacker
	}

	from, err := h.Sender(ele)
	if err != nil {
		return err
	}

	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}

	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}

	currentBlock := blockChain.CurrentBlock()

	if currentState.GetTradableBalance(from, currentBlock.Header.Round).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}

	intrGas, err := txexec.IntrinsicGas(tx.Data(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}

	if currentBlock.Header.GasLimit < tx.Gas() {
		return ErrGasLimit
	}

	if stateDB, _, ok := p.GetStateBeforeCacheHeight(); ok {
		if stateDB.GetNonce(from) > ele.Nonce() {
			return pool.ErrNonceTooLow
		}
	}

	return nil
}

func (h *TxHelper) Sender(ele pool.Element) (common.Address, error) {
	tx := (ele).(*types.Transaction)
	return h.signer.Sender(tx)
}

func NewTxPool(conf *config.Config, c *chain.BlockChain) pool.Pool {
	s := types.MakeSigner(conf.ChainConfig.TxSignerType, conf.ChainConfig.ChainID)
	helper := &TxHelper{
		signer: s,
	}
	if conf.TxPoolConfig.FakeMode {
		return pool.NewFakePool(conf.TxPoolConfig.StartCleanTime, conf.TxPoolConfig.CleanPeriod, conf.TxPoolConfig.LeftEleNumEachAddr, helper)
	}
	return pool.NewPool(*conf.TxPoolConfig, c, TransactionType, helper)
}

func ElemsToTxs(elems []pool.Element) []*types.Transaction {
	if len(elems) == 0 {
		return make([]*types.Transaction, 0)
	}
	if !IsTx(elems[0]) {
		log.Error("the element type is not *types.Transaction.", "element", elems[0]) // should never happen.
		return nil
	}
	txs := make([]*types.Transaction, len(elems))
	for i, elem := range elems {
		txs[i] = elem.(*types.Transaction)
	}
	return txs
}

func IsTx(elem pool.Element) bool {
	_, ok := elem.(*types.Transaction)
	return ok
}
