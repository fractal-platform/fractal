package packer

import (
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/event"
)

type Packer interface {
	//
	InsertRemoteTxPackage(pkg *types.TxPackage) error

	// pack_service
	InsertTransactions(txs types.Transactions) []error
	StartPacking(packerIndex uint32)
	StopPacking()
	IsPacking() bool
	Subscribe(ch chan<- types.TxPackages) event.Subscription
}
