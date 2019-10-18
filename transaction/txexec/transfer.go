// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package txexec implements all transaction executors.
package txexec

import (
	"math/big"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/state"
)

// CanTransfer checks whether there are enough funds in the address' account to make a transfer.
// This does not take the necessary gas in to account to make the transfer valid.
func CanTransfer(db *state.StateDB, addr common.Address, amount *big.Int) bool {
	return db.GetBalance(addr).Cmp(amount) >= 0
}

// Transfer subtracts amount from sender and adds amount to recipient using the given Db
func Transfer(db *state.StateDB, sender, recipient common.Address, amount *big.Int) {
	db.SubBalance(sender, amount)
	db.AddBalance(recipient, amount)
}
