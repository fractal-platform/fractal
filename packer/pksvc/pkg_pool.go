package pksvc

import (
	"errors"
	"github.com/fractal-platform/fractal/params"
	"reflect"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

var TxPackageType = reflect.TypeOf(types.TxPackage{})

var (
	ErrBlockNotFound = errors.New("Block not found")
	ErrPackageTooOld = errors.New("Package is too old")
)

type PkgHelper struct {
}

func (h *PkgHelper) Reset(pool pool.Pool, block *types.Block) {
	//do nothing
}

func (h *PkgHelper) Validate(p pool.Pool, ele pool.Element, currentState pool.StateDB, chain pool.BlockChain) error {
	if stateDB, _, ok := p.GetStateBeforeCacheHeight(); ok {
		from, _ := h.Sender(ele)
		if stateDB.GetPackageNonce(from) > ele.Nonce() {
			return pool.ErrNonceTooLow
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

func (h *PkgHelper) Sender(ele pool.Element) (common.Address, error) {
	txPackage := ele.(*types.TxPackage)
	return txPackage.Packer(), nil
}

func NewPkgPool(conf config.PoolConfig, c *chain.BlockChain) pool.Pool {
	if conf.FakeMode {
		return pool.NewFakePool(conf.StartCleanTime, conf.CleanPeriod, conf.LeftEleNumEachAddr, &PkgHelper{})
	}
	return pool.NewPool(conf, c, TxPackageType, &PkgHelper{})
}

func ElemsToTxPkgs(elems []pool.Element) []*types.TxPackage {
	if len(elems) == 0 {
		return nil
	}
	if _, ok := elems[0].(*types.TxPackage); !ok {
		log.Error("the element type is not *types.TxPackage.", "element", elems[0]) // should never happen.
		return nil
	}
	txPkgs := make([]*types.TxPackage, len(elems))
	for i, elem := range elems {
		txPkgs[i] = elem.(*types.TxPackage)
	}
	return txPkgs
}

func ElemsToTxPkgHashes(elems []pool.Element) []common.Hash {
	if len(elems) == 0 {
		return nil
	}
	if _, ok := elems[0].(*types.TxPackage); !ok {
		log.Error("the element type is not *types.TxPackage.", "element", elems[0]) // should never happen.
		return nil
	}
	txPkgHashes := make([]common.Hash, len(elems))
	for i, elem := range elems {
		txPkgHashes[i] = elem.(*types.TxPackage).Hash()
	}
	return txPkgHashes
}
