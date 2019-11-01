package pksvc

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/packer"
)

const (
	DefaultPkgSize = 1024
)

type blockChain interface {
	Database() dbwrapper.Database
	InsertTxPackage(pkg *types.TxPackage)
	CurrentBlock() *types.Block
	StateAt(root common.Hash) (*state.StateDB, error)
	GetPrePackerNumber(headBlockWhenPacking *types.Block) (uint32, error)
	GetPrePackerInfoByIndex(headBlockWhenPacking *types.Block, index uint32) (*types.PackerInfo, *types.Block, error)
	GetBlock(blockHash common.Hash) *types.Block
}

type packerKeyManager interface {
	GetPrivateKey(address common.Address, pubkey types.PackerECPubKey) (crypto.PrivateKey, error)
}

func NewPacker(cfg *config.Config, pkgPool pool.Pool, packerKeyManager packerKeyManager, txSigner types.Signer, chain blockChain, packerGroupSize uint64) packer.Packer {
	packService := newPackService(cfg, packerKeyManager, pkgPool, txSigner, chain, packerGroupSize)
	return packService
}
