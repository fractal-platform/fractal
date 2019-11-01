package api

import (
	"context"
	"math/big"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/ftl/sync"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/logbloom/bloomquery"
	"github.com/fractal-platform/fractal/packer"
)

type fractal interface {
	IsMining() bool
	StartMining() error
	StopMining()
	MiningKeyManager() *keys.MiningKeyManager
	Coinbase() common.Address

	Config() *config.Config
	Packer() packer.Packer
	BlockChain() *chain.BlockChain
	TxPool() pool.Pool
	Signer() types.Signer
	GasPrice() *big.Int
	GetPoolTransactions() types.Transactions

	FtlVersion() int

	Synchronizer() *sync.Synchronizer

	ChainDb() dbwrapper.Database
	GetBlock(ctx context.Context, fullHash common.Hash) *types.Block
	CurrentBlock(ctx context.Context) *types.Block
	GetBlockStr(blockStr string) *types.Block
	GetReceipts(ctx context.Context, blockHash common.Hash) types.Receipts
	GetLogs(ctx context.Context, blockHash common.Hash) [][]*types.Log

	GetMainBranchBlock(height uint64) (*types.Block, error)
	BloomRequestsReceiver() chan chan *bloomquery.Retrieval
	SubscribeInsertBloomEvent(ch chan<- types.BloomInsertEvent) event.Subscription
}
