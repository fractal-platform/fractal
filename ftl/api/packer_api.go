// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"context"
	"errors"
	"github.com/fractal-platform/fractal/chain"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/rlp"
)

type PackerAPI struct {
	chain  *chain.BlockChain
	packer packer.Packer
}

func NewPackerAPI(chain *chain.BlockChain, packer packer.Packer) *PackerAPI {
	return &PackerAPI{chain, packer}
}

func (s *PackerAPI) GetTxPackageByHash(hash common.Hash) *types.TxPackage {
	return s.chain.GetTxPackage(hash)
}

func (s *PackerAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}
	if errs := s.packer.InsertTransactions(types.Transactions{tx}); errs[0] != nil {
		return common.Hash{}, errs[0]
	}

	return tx.Hash(), nil
}

func (s *PackerAPI) SendRawTransactions(ctx context.Context, encodedTxs hexutil.Bytes) ([]string, error) {
	txs := types.Transactions{}
	if err := rlp.DecodeBytes(encodedTxs, &txs); err != nil {
		return nil, err
	}
	errs := s.packer.InsertTransactions(txs)

	var packErr error
	var packErrStrings = make([]string, len(errs))

	for i, err := range errs {
		if err != nil {
			packErrStrings[i] = err.Error()
			packErr = errors.New("packer.InsertTransactions error")
		}
	}

	return packErrStrings, packErr
}
