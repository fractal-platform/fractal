package pool

import (
	"errors"
	"reflect"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
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
)

type txHelper struct {
	signer types.Signer
}

func (h *txHelper) reset(p Pool, newHead *types.Block) {
	// Traversing the transactions in the block, comparing the nonce
	for _, newTx := range newHead.Body.Transactions {
		address, _ := types.Sender(h.signer, newTx)
		if newTx.Nonce() >= p.StateUnsafe().GetNonce(address) {
			// Put into the p, this happens only for non-local blocks
			p.AddUnsafe([]Element{newTx}, false)
		}
	}
}

func (h *txHelper) validate(p Pool, ele Element, currentState StateDB, blockChain BlockChain) error {
	tx := (ele).(*types.Transaction)

	if !tx.Broadcast() {
		return ErrIsNotAPacker
	}

	from, err := h.sender(ele)
	if err != nil {
		return err
	}

	if tx.Size() > 32*1024 {
		return ErrOversizedData
	}

	if tx.Value().Sign() < 0 {
		return ErrNegativeValue
	}

	if currentState.GetBalance(from).Cmp(tx.Cost()) < 0 {
		return ErrInsufficientFunds
	}

	intrGas, err := txexec.IntrinsicGas(tx.Data(), tx.To() == nil)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return ErrIntrinsicGas
	}

	if blockChain.CurrentBlock().Header.GasLimit < tx.Gas() {
		return ErrGasLimit
	}

	if stateDB, _, ok := p.GetStateBeforeCacheHeight(); ok {
		if stateDB.GetNonce(from) > ele.Nonce() {
			return ErrNonceTooLow
		}
	}

	return nil
}

func (h *txHelper) sender(ele Element) (common.Address, error) {
	tx := (ele).(*types.Transaction)
	return types.Sender(h.signer, tx)
}

func NewTxPool(conf *config.Config, c *chain.BlockChain) Pool {
	s := types.MakeSigner(conf.ChainConfig.TxSignerType, conf.ChainConfig.ChainID)
	helper := &txHelper{
		signer: s,
	}
	if conf.TxPoolConfig.FakeMode {
		return NewFakePool(conf.TxPoolConfig.StartCleanTime, conf.TxPoolConfig.CleanPeriod, conf.TxPoolConfig.LeftEleNumEachAddr, helper)
	}
	return NewPool(*conf.TxPoolConfig, c, TransactionType, helper)
}

func ElemsToTxs(elems []Element) []*types.Transaction {
	if len(elems) == 0 {
		return make([]*types.Transaction, 0)
	}

	if _, ok := elems[0].(*types.Transaction); !ok {
		log.Error("the element type is not *types.Transaction.", "element", elems[0]) // should never happen.
		return nil
	}
	txs := make([]*types.Transaction, len(elems))
	for i, elem := range elems {
		txs[i] = elem.(*types.Transaction)
	}
	return txs
}
