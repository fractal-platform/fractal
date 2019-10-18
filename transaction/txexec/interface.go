// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package txexec implements all transaction executors.
package txexec

import (
	"errors"
	"sync"

	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	ErrNonceNotAllowed           = errors.New("nonce not allowed")
	ErrNonceNotAllowedTooNew     = errors.New("nonce not allowed(too new)")
	ErrNonceNotAllowedContained  = errors.New("nonce not allowed(contained)")
	ErrNonceNotAllowedTooOld     = errors.New("nonce not allowed(too old)")
	ErrNonceSetNotFound          = errors.New("nonceSet not found")
	ErrInsufficientBalance       = errors.New("insufficient balance")
	ErrSimpleTxHasNoTarget       = errors.New("simple transaction has no target")
	ErrInsufficientBalanceForGas = errors.New("insufficient balance to pay for gas")
	ErrOutOfGas                  = errors.New("out of gas")
	ErrCodeStoreOutOfGas         = errors.New("contract creation code storage out of gas")
	ErrWasmExec                  = errors.New("wasm exec return error")
)

type TxExecutor interface {
	ExecuteTxPackages(txpkgs types.TxPackages, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int)
	ExecuteTransactions(txs types.Transactions, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, txPackageIndex uint32, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int)
}

func NewExecutor(exeType string, maxBitLength uint64, signer types.Signer) TxExecutor {
	switch exeType {
	case "wasm":
		return NewWasmExecutor(signer, maxBitLength)
	case "simple":
		return NewSimpleExecutor(signer, maxBitLength)
	case "dumb":
		return &dumbExecutor{}
	}

	// if no special config, make sure everything is ok.
	return NewWasmExecutor(signer, maxBitLength)
}

type TxExecResult struct {
	StateDb  *state.StateDB
	Receipts types.Receipts
}

type dumbExecutor struct {
}

func (*dumbExecutor) ExecuteTxPackages(txpkgs types.TxPackages, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	return nil, nil, nil, len(block.Body.TxPackageHashes)
}

func (*dumbExecutor) ExecuteTransactions(txs types.Transactions, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, txPackageIndex uint32, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	return nil, nil, nil, len(txs)
}

func CacheTxSender(txs types.Transactions, signer types.Signer, threadNum int) {
	txChan := make(chan *types.Transaction)
	var wg sync.WaitGroup

	wg.Add(threadNum)
	for i := 0; i < threadNum; i++ {
		go func() {
			for tx := range txChan {
				_, err := types.Sender(signer, tx)
				if err != nil {
					log.Error("CacheTxSender: get tx sender error", "err", err)
				}
			}
			wg.Done()
		}()
	}

	for _, tx := range txs {
		txChan <- tx
	}
	close(txChan)
	wg.Wait()
}
