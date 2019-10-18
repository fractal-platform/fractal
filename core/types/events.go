// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package types contains data types related to Fractal consensus.
package types

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*Transaction }

type ChainUpdateEvent struct{ Block *Block }

type NewMinedBlockEvent struct{ Block *Block }

type BlockExecutedEvent struct{ Block *Block }
