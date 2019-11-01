package pool

import (
	"errors"
	"reflect"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
)

var TxPackageType = reflect.TypeOf(types.TxPackage{})

var (
	ErrBlockNotFound = errors.New("Block not found")
	ErrPackageTooOld = errors.New("Package is too old")
)

type pkgHelper struct{}

func (h *pkgHelper) reset(pool Pool, block *types.Block) {
	//do nothing
}

func (h *pkgHelper) validate(p Pool, ele Element, currentState StateDB, chain BlockChain) error {
	if stateDB, _, ok := p.GetStateBeforeCacheHeight(); ok {
		from, _ := h.sender(ele)
		if stateDB.GetPackageNonce(from) > ele.Nonce() {
			return ErrNonceTooLow
		}
	}

	// if the package is too old
	relateBlock := chain.GetBlock(ele.(*types.TxPackage).BlockFullHash())
	if relateBlock == nil {
		return ErrBlockNotFound
	}
	packingBlockHeight := relateBlock.Header.Height + uint64(chain.GetGreedy()) + params.PackerKeyConfirmDistance
	minHeight, err := chain.MinAvailablePackageHeight()
	if err != nil {
		return err
	}
	if packingBlockHeight < minHeight {
		return ErrPackageTooOld
	}

	return nil
}

func (h *pkgHelper) sender(ele Element) (common.Address, error) {
	txPackage := ele.(*types.TxPackage)
	return txPackage.Packer(), nil
}

func NewPkgPool(conf config.PoolConfig, c *chain.BlockChain) Pool {
	if conf.FakeMode {
		return NewFakePool(conf.StartCleanTime, conf.CleanPeriod, conf.LeftEleNumEachAddr, &pkgHelper{})
	}
	return NewPool(conf, c, TxPackageType, &pkgHelper{})
}
