package chain

import (
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
)

const (
	TxVerifyParallel = 16
)

func (bc *BlockChain) HasTxPackage(pkgHash common.Hash) bool {
	if bc.pkgCache.Contains(pkgHash) {
		return true
	}
	return dbaccessor.HasTxPkg(bc.db, pkgHash)
}

func (bc *BlockChain) GetTxPackage(pkgHash common.Hash) *types.TxPackage {
	if pkg, ok := bc.pkgCache.Get(pkgHash); ok {
		return pkg.(*types.TxPackage)
	}

	pkg, err := dbaccessor.ReadTxPkg(bc.db, pkgHash)
	if err != nil {
		//bc.logger.Error("Read tx package from db failed", "err", err)
		return nil
	}
	return pkg
}

func (bc *BlockChain) GetTxPackageList(hashes []common.Hash) types.TxPackages {
	var pkgList types.TxPackages
	for _, pkgHash := range hashes {
		pkg := bc.GetTxPackage(pkgHash)
		if pkg != nil {
			pkgList = append(pkgList, pkg)
		}
	}
	return pkgList
}

func (bc *BlockChain) VerifyTxPackage(pkg *types.TxPackage) error {
	var pubKey types.PackerECPubKey

	pubKeySlice, err := bc.pkgSigner.RecoverPubKey(pkg)
	if err != nil {
		return err
	}
	copy(pubKey[:], pubKeySlice)

	blockWhenPacking := bc.GetBlock(pkg.BlockFullHash())
	if blockWhenPacking == nil {
		bc.logger.Error("block not found when verify tx", "blockHash", pkg.BlockFullHash(), "pkgHash", pkg.Hash())
		// put into future map
		bc.addFutureBlockTxPackage(pkg.BlockFullHash(), pkg)
		// return a special error to catch
		return ErrTxPackageRelatedBlockNotFound
	}

	packerIndex, _, _, err := bc.GetPackerInfoByPubKey(blockWhenPacking, pubKey)
	if err != nil {
		bc.addFutureBlockTxPackage(pkg.BlockFullHash(), pkg)
		bc.logger.Error("Verify tx package failed", "err", err, "pkgHash", pkg.Hash(), "blockHash", pkg.BlockFullHash())
		return ErrTxPackageRelatedBlockNotFound
	}

	var wg sync.WaitGroup
	txs := pkg.Transactions()
	current := 0
	left := len(txs)
	errs := make([]error, TxVerifyParallel)
	for i := 0; i < TxVerifyParallel; i++ {
		batch := left / (TxVerifyParallel - i)
		if left%(TxVerifyParallel-i) > 0 {
			batch += 1
		}
		txlist := txs[current : current+batch]
		current += batch
		left -= batch

		wg.Add(1)
		go func(index int, txlist []*types.Transaction) {
			defer wg.Done()
			for _, tx := range txlist {
				// TODO: PACKER txSize
				if tx.Broadcast() {
					errs[index] = ErrIsBroadcastTx
					return
				}

				// Whether the transaction and the packer match
				if !tx.MatchPacker(bc.chainConfig.PackerGroupSize, packerIndex, bc.txSigner) {
					errs[index] = ErrTransactionNotMatchPacker
					return
				}

				if _, err := types.Sender(bc.txSigner, tx); err != nil {
					errs[index] = err
					return
				}
			}
		}(i, txlist)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

func (bc *BlockChain) InsertTxPackage(pkg *types.TxPackage) {
	dbaccessor.WriteTxPkg(bc.db, pkg)
	bc.pkgCache.Add(pkg.Hash(), pkg)

	// process future block
	for _, block := range bc.getFutureTxPackageBlocks(pkg.Hash()) {
		bc.logger.Info("Process future txPackage block", "txPkgHash", pkg.Hash(), "blockHash", block.FullHash())
		bc.futureBlockFeed.Send(types.FutureBlockEvent{Block: block})
	}
	bc.removeFutureTxPackageBlocks(pkg.Hash())
}

// TODO: validate pkg by ancestor-block
func (bc *BlockChain) ValidatePackage(pkg *types.TxPackage, height uint64) error {
	minPkgHeight := MinPkgHeightAllowedToPutIntoTheBlock(height)
	maxPkgHeight := MaxPkgHeightAllowedToPutIntoTheBlock(height)

	relatedBlock := bc.GetBlock(pkg.BlockFullHash())
	if relatedBlock == nil {
		return ErrBlockNotFound
	}

	pkgHeight := relatedBlock.Header.Height + uint64(bc.GetGreedy()) + params.PackerKeyConfirmDistance
	if pkgHeight < minPkgHeight {
		return ErrPackageHeightTooLow
	}
	if pkgHeight > maxPkgHeight {
		return ErrPackageHeightTooHigh
	}
	return nil
}

// Return the min height: Packages with a lower height will not be packaged by the next mined block
func (bc *BlockChain) MinAvailablePackageHeight() (uint64, error) {
	currentBlock := bc.CurrentBlock()
	if currentBlock == nil {
		return 0, ErrBlockNotFound
	}

	greedy := uint64(bc.GetGreedy())

	var minNextMinedBlockHeight uint64
	if currentBlock.Header.Height < greedy {
		minNextMinedBlockHeight = 1
	} else {
		minNextMinedBlockHeight = currentBlock.Header.Height - greedy + 1
	}

	return MinPkgHeightAllowedToPutIntoTheBlock(minNextMinedBlockHeight), nil
}

// The minimum height of the block that the package is allowed to be placed in
func MinBlockHeightAllowedToCarryThePkg(txPkgHeight uint64) uint64 {
	return txPkgHeight + MinPackageHeightDelay
}

func MaxBlockHeightAllowedToCarryThePkg(txPkgHeight uint64) uint64 {
	return txPkgHeight + MaxPackageHeightDelay
}

// The minimum height of the package that allowed to put into the block
func MinPkgHeightAllowedToPutIntoTheBlock(blockHeight uint64) uint64 {
	if blockHeight >= MaxPackageHeightDelay {
		return blockHeight - MaxPackageHeightDelay
	}
	return 0
}

func MaxPkgHeightAllowedToPutIntoTheBlock(blockHeight uint64) uint64 {
	if blockHeight >= MinPackageHeightDelay {
		return blockHeight - MinPackageHeightDelay
	}
	return 0
}
