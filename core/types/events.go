// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package types contains data types related to Fractal consensus.
package types

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*Transaction }

type ChainUpdateEvent struct{ Block *Block }

type NewMinedBlockEvent struct{ Block *Block }

type BlockExecutedEvent struct{ Block *Block }

type BloomInsertEvent struct{ Block *Block }

// future block which is sent from blockchain, and will be processed in network handler
type FutureBlockEvent struct{ Block *Block }

// future txpkg which is sent from blockchain, and will be processed in network handler
type FutureTxPackageEvent struct{ Pkg *TxPackage }
