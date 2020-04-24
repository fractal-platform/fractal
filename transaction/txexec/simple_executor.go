// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package txexec implements all transaction executors.
package txexec

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

type SimpleExecutor struct {
	signer       types.Signer
	maxBitLength uint64
}

func NewSimpleExecutor(signer types.Signer, maxBitLength uint64) TxExecutor {
	log.Info("NewExecutor: Init SimpleExecutor")
	return &SimpleExecutor{
		signer:       signer,
		maxBitLength: maxBitLength,
	}
}

func (e *SimpleExecutor) ExecuteTxPackages(txpkgs types.TxPackages, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	var (
		newExecutedTxs  = executedTxs
		newAllLogs      = allLogs
		newReceipts     = receipts
		gasLimitReached bool

		loopEndIndex = len(txpkgs)
	)

	for i, txpkg := range txpkgs {
		newExecutedTxs, newAllLogs, newReceipts, gasLimitReached = e.ExecuteTxPackage(uint32(i), txpkg, prevStateDb, state, newReceipts, block, newExecutedTxs, usedGas, newAllLogs, gasPool, callbackParamKey)

		if gasLimitReached {
			loopEndIndex = i
			log.Warn("ExecuteTxPackages: reach block gas limit", "the last pkg index", loopEndIndex)
			break
		}
	}

	return newExecutedTxs, newAllLogs, newReceipts, loopEndIndex
}

func (e *SimpleExecutor) ExecuteTxPackage(txPackageIndex uint32, txPackage *types.TxPackage, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, bool) {
	// package snapshot for revert
	oldState := state.Copy()
	oldGasPool := *gasPool
	oldUsedGas := *usedGas

	nonceSet := state.PackageNonceSet(txPackage.Packer())
	if nonceSet == nil {
		log.Error("cannot find package nonce set", "addr", txPackage.Packer())
		return nil, nil, nil, false
	}

	var (
		needReset     bool
		newStartNonce uint64
	)
	if prevStateDb != nil {
		needReset = true
		newStartNonce = prevStateDb.GetPackageNonce(txPackage.Packer())
	} else {
		log.Warn("prevStateDb error")
	}

	log.Info("ExecuteTxPackage pkg nonce", "pkgNonce", state.GetPackageNonce(txPackage.Packer()))
	searchResult, err, resetChanged := nonceSet.ResetThenSearch(needReset, newStartNonce, txPackage.Nonce(), e.maxBitLength)
	if err != nil {
		log.Error("ExecuteTxPackage error: ResetThenSearch Error", "err", err)
		return nil, nil, nil, false
	}

	if searchResult != nonces.NotContainedAndAllowed {
		log.Info("ExecuteTxPackage error: NonceError", "searchResult", searchResult, "txPackage.Nonce", txPackage.Nonce(), "txPackage.Packer", txPackage.Packer())
		return nil, nil, nil, false
	}

	newExecutedTxs, newAllLogs, newReceipts, loopEndIndex := e.ExecuteTransactions(txPackage.Transactions(), prevStateDb, state, receipts, block, txPackageIndex, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)

	if loopEndIndex < len(txPackage.Transactions()) {
		// revert
		state.ResetTo(oldState)
		*gasPool = oldGasPool
		*usedGas = oldUsedGas
		return executedTxs, allLogs, receipts, true
	}

	_, addChanged := nonceSet.Add(txPackage.Nonce(), e.maxBitLength)

	if resetChanged || addChanged {
		state.MarkPackageNonceSetJournal(txPackage.Packer())
		state.FinaliseOne()
	}

	log.Info("ExecuteTxPackage success", "txPackage.Nonce", txPackage.Nonce(), "txPackage.Packer", txPackage.Packer())
	return newExecutedTxs, newAllLogs, newReceipts, false
}

func (e *SimpleExecutor) ExecuteTransactions(txs types.Transactions, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, txPackageIndex uint32, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	var (
		newExecutedTxs = executedTxs
		newAllLogs     = allLogs
		newReceipts    = receipts

		loopEndIndex = len(txs)
	)

	for i, tx := range txs {
		_, _, err := e.ExecuteTransaction(prevStateDb, state, tx, block, gasPool, usedGas, callbackParamKey)
		if err == nil {
			// Everything ok, collect the logs and shift in the next transaction from the same account
			newExecutedTxs = append(newExecutedTxs, &types.TxWithIndex{Tx: tx, TxPackageIndex: txPackageIndex, TxIndex: uint32(i)})
		} else if err == types.ErrGasLimitReached {
			loopEndIndex = i
			log.Warn("ExecuteTransactions: reach block gas limit", "the last tx index", loopEndIndex)
			break
		}
	}
	log.Debug("ExecuteTransactions: exec transactions", "num", len(newExecutedTxs))

	return newExecutedTxs, newAllLogs, newReceipts, loopEndIndex
}

func (e *SimpleExecutor) ExecuteTransaction(prevStateDb *state.StateDB, state *state.StateDB, tx *types.Transaction, block *types.Block, gp *types.GasPool, usedGas *uint64, callbackParamKey uint64) (*types.Receipt, common.Address, error) {
	snap := state.Snapshot()

	_, _, from, err := e.ApplyTransaction(prevStateDb, state, tx, block, gp, usedGas, callbackParamKey)

	if err != nil {
		state.RevertToSnapshot(snap)
		return nil, common.Address{}, err
	}

	state.FinaliseOne()

	return nil, from, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func (e *SimpleExecutor) ApplyTransaction(prevStateDb *state.StateDB, state *state.StateDB, tx *types.Transaction, block *types.Block, gp *types.GasPool, usedGas *uint64, callbackParamKey uint64) (*types.Receipt, uint64, common.Address, error) {
	msg, err := tx.AsMessage(e.signer)
	if err != nil {
		return nil, 0, common.Address{}, err
	}
	_, useGas, err := SimpleApplyMessage(prevStateDb, state, block.Header.Round, msg, gp, e.maxBitLength, block.Header.Coinbase, callbackParamKey)

	if err != nil {
		log.Info("ApplyTransaction err", "from", msg.From(), "to", msg.To(), "nonce", msg.Nonce(), "hash", tx.Hash(), "err", err)
		return nil, 0, common.Address{}, err
	}
	//log.Debug("ApplyTransaction ok", "from", msg.From(), "nonce", msg.Nonce(), "useGas", useGas)
	*usedGas += useGas

	return nil, useGas, msg.From(), nil
}

func SimpleApplyMessage(prevStateDb *state.StateDB, statedb *state.StateDB, round uint64, msg Message, gp *types.GasPool, maxBitLength uint64, coinbase common.Address, callbackParamKey uint64) ([]byte, uint64, error) {
	nonceSet := statedb.TxNonceSet(msg.From())
	if nonceSet == nil {
		log.Error("SimpleApplyMessage: cannot find tx nonce set", "addr", msg.From())
		return nil, 0, ErrNonceSetNotFound
	}

	return NewStateTransition(prevStateDb, statedb, round, msg, gp, nonceSet, maxBitLength, callbackParamKey).SimpleTransitionDb(coinbase)
}
