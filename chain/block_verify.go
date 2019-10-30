package chain

import (
	"math/big"
	"time"

	"github.com/deckarep/golang-set"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/diffculty"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/params"
)

func (bc *BlockChain) VerifyBlock(block *types.Block, checkGreedy bool) (types.Blocks, common.Hash, common.Hash, error) {
	bc.logger.Info("Block verify start", "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	// TODO: checkpoint also return nil
	if block.FullHash() == bc.genesisBlock.FullHash() {
		return nil, common.Hash{}, common.Hash{}, nil
	}

	// check depend block exists
	dependBlockHash, err := bc.VerifyBlockDepend(block)
	if err != nil {
		bc.addFutureBlock(dependBlockHash, block)
		bc.logger.Error("Block verify failed", "hash", block.FullHash(), "miss", dependBlockHash, "err", err)
		return nil, dependBlockHash, common.Hash{}, err
	}

	// check confirm blocks
	confirmBlocks, err := bc.verifyConfirmBlocks(block)
	if err != nil {
		return nil, dependBlockHash, common.Hash{}, err
	}

	// * Whether the parent node exists and is confirmed
	parentBlock := bc.GetBlock(block.Header.ParentFullHash)
	if parentBlock.SimpleHash() != block.Header.ParentHash {
		bc.logger.Error("Block verify failed", "err", ErrNotConfirmParentBlock)
		return nil, common.Hash{}, common.Hash{}, ErrNotConfirmParentBlock
	}

	// TODO: we skip low-height block, because we can't handle it very well
	//if bc.CurrentBlock().Header.Height > block.Header.Height + 10 {
	//	return nil, common.Hash{}, common.Hash{}, ErrBlockHeightTooLow
	//}

	// * Whether the block meets greedy rules
	currentBlock := bc.CurrentBlock()
	// If the height of the new block is greater than the height of the current main chain,
	// then it will become the new main chain, and there is no need to judge greedy.
	if block.Header.Height <= currentBlock.Header.Height && checkGreedy {
		// use greedy+5 to accept blocks not very-far-away
		check, err := bc.CheckGreedy(block, currentBlock, uint64(bc.chainConfig.Greedy)+5)
		if err != nil {
			return nil, common.Hash{}, common.Hash{}, err
		}
		// adjust greedy constrains to (greedy+5)
		if !check {
			bc.logger.Error("Block verify failed", "err", ErrBlockNotMeetGreedy, "head", currentBlock.FullHash(), "block", block.FullHash())
			return nil, common.Hash{}, common.Hash{}, ErrBlockNotMeetGreedy
		}
	}
	bc.logger.Info("Block verify greedy OK", "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	// Verify that the gas limit is <= 2^63-1
	if block.Header.GasLimit > params.MaxGasLimit {
		bc.logger.Error("Block verify failed", "err", ErrInvalidGasLimit)
		return nil, common.Hash{}, common.Hash{}, ErrInvalidGasLimit
	}
	// Verify that the gasUsed is <= gasLimit
	if block.Header.GasUsed > block.Header.GasLimit {
		bc.logger.Error("Block verify failed", "err", ErrInvalidGasUsed)
		return nil, common.Hash{}, common.Hash{}, ErrInvalidGasUsed
	}

	// Verify that the gas limit remains within allowed bounds
	diff := int64(parentBlock.Header.GasLimit) - int64(block.Header.GasLimit)
	if diff < 0 {
		diff *= -1
	}
	limit := parentBlock.Header.GasLimit / params.GasLimitBoundDivisor

	if uint64(diff) >= limit || block.Header.GasLimit < params.MinGasLimit {
		bc.logger.Error("Block verify failed", "err", ErrInvalidGasLimit)
		return nil, common.Hash{}, common.Hash{}, ErrInvalidGasLimit
	}

	// * Confirm Height and Round
	if block.Header.Height != parentBlock.Header.Height+1 {
		bc.logger.Error("Block verify failed", "err", ErrBlockHeightError)
		return nil, common.Hash{}, common.Hash{}, ErrBlockHeightError
	}
	if block.Header.Round <= parentBlock.Header.Round {
		bc.logger.Error("Block verify failed", "err", ErrBlockRoundTooLow)
		return nil, common.Hash{}, common.Hash{}, ErrBlockRoundTooLow
	}

	// Fetch stake&pubkey from pre state
	balance, pubkey, err := bc.GetPreBalanceAndPubkey(parentBlock, block.Header.Coinbase)
	if err != nil {
		bc.logger.Error("Get balance and pubkey failed", "parentBlock", parentBlock.FullHash(), "coinbase", block.Header.Coinbase)
		return nil, common.Hash{}, common.Hash{}, err
	}

	// * Whether the block meets the consensus
	maxUint256 := new(big.Int).Exp(big.NewInt(2), big.NewInt(256), big.NewInt(0))
	expected := difficulty.CalcDifficulty(block.Header.Round, parentBlock.Header.Round, parentBlock.Header.Difficulty)
	if expected.Cmp(block.Header.Difficulty) != 0 {
		bc.logger.Error("difficulty compare failed", "calcDifficulty", expected, "blockDifficulty", block.Header.Difficulty)
		return nil, common.Hash{}, common.Hash{}, ErrBlockConsensusError
	}
	target := new(big.Int).Div(new(big.Int).Mul(new(big.Int).SetUint64(balance), maxUint256), block.Header.Difficulty)
	if new(big.Int).SetBytes(block.SimpleHash().Bytes()).Cmp(target) > 0 {
		return nil, common.Hash{}, common.Hash{}, ErrBlockConsensusError
	}

	// Whether we ignore the sig verify
	if !bc.chainConfig.BlockSigFake {
		// * Verify Sig[]
		key, err := crypto.UnmarshalPubKey(crypto.BLS, pubkey)
		if err != nil {
			return nil, common.Hash{}, common.Hash{}, err
		}
		if !key.Verify(block.SignHashByte(), block.Header.Sig) {
			return nil, common.Hash{}, common.Hash{}, ErrBlockSigError
		}

		// * Verify FullSig[]
		if !key.Verify(block.FullHash().Bytes(), block.Header.FullSig) {
			return nil, common.Hash{}, common.Hash{}, ErrBlockFullSigError
		}
	}
	bc.logger.Info("Block verify hash function OK", "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	for _, tx := range block.Body.Transactions {
		_, err := types.Sender(bc.txSigner, tx)
		if err != nil {
			return nil, common.Hash{}, common.Hash{}, ErrPackTxSignError
		}
	}

	// * Whether the TxHash is correct
	if block.Header.TxHash != types.DeriveSha(types.Transactions(block.Body.Transactions)) {
		bc.logger.Error("Block verify failed", "err", ErrBlockTxHashError)
		return nil, common.Hash{}, common.Hash{}, ErrBlockTxHashError
	}

	// * TODO: need to add a TxPackagesHash = (DeriveSha([]TxPackageHashes)

	// * Whether the TxPackage has received
	txPackageHashes := block.Body.TxPackageHashes
	for i := range txPackageHashes {
		pkg := bc.GetTxPackage(txPackageHashes[i])
		if pkg == nil {
			bc.addFutureTxPackageBlock(txPackageHashes[i], block)
			bc.logger.Error("Block verify failed", "miss", txPackageHashes[i], "err", ErrBlockTxPackageMissing)
			return nil, common.Hash{}, txPackageHashes[i], ErrBlockTxPackageMissing
		}
		err = bc.ValidatePackage(pkg, block.Header.Height)
		if err != nil {
			bc.logger.Error("Block verify failed", "relateBlockHash", pkg.BlockFullHash(), "pkgHash", pkg.Hash(), "err", err)
			return nil, common.Hash{}, common.Hash{}, err
		}
	}

	bc.logger.Info("Block verify OK", "hash", block.FullHash(), "duration", common.PrettyDuration(time.Since(block.ReceivedAt)))

	return confirmBlocks, common.Hash{}, common.Hash{}, nil
}

func (bc *BlockChain) VerifyBlockDepend(block *types.Block) (common.Hash, error) {
	// TODO: checkpoint also return nil
	if block.FullHash() == bc.genesisBlock.FullHash() {
		return common.Hash{}, nil
	}

	// * Whether the parent node exists and is confirmed
	parentBlock := bc.GetBlock(block.Header.ParentFullHash)
	if parentBlock == nil {
		// TODO what if someone else malicious send invalid block
		return block.Header.ParentFullHash, ErrCannotFindParentBlock
	}

	for _, fullHash := range block.Header.Confirms {
		// Whether the confirmed block exists
		var confirmBlock = bc.GetBlock(fullHash)
		if confirmBlock == nil {
			return fullHash, ErrConfirmUnknownBlock
		}
	}
	return common.Hash{}, nil
}

func (bc *BlockChain) verifyConfirmBlocks(block *types.Block) (types.Blocks, error) {
	// for the blocks it confirmed
	var confirmBlocks types.Blocks

	// find parent and grand parent
	var grandParentBlock *types.Block
	var parentBlock = bc.GetBlock(block.Header.ParentFullHash)
	if block.Header.Height > 1 {
		grandParentBlock = bc.GetBlock(parentBlock.Header.ParentFullHash)
		if grandParentBlock == nil {
			return nil, ErrCannotFindGrandparentBlock
		}
	}

	// use a set to check if has duplicated simple hash
	confirmedBlockSimpleHashSet := mapset.NewSet()

	for _, fullHash := range block.Header.Confirms {
		// Whether the round of confirmed block is in a correct range(hash)
		var confirmBlock = bc.GetBlock(fullHash)

		// check if has duplicated simple hash
		if confirmedBlockSimpleHashSet.Contains(confirmBlock.SimpleHash()) {
			return nil, ErrConfirmedBlockHasSameSimpleHash
		}

		if confirmBlock.CompareByRoundAndSimpleHash(parentBlock) >= 0 {
			return nil, ErrConfirmBlockNotMeetRound
		}
		if grandParentBlock != nil && confirmBlock.CompareByRoundAndSimpleHash(grandParentBlock) <= 0 {
			return nil, ErrConfirmBlockNotMeetRound
		}

		// Whether the confirmed block meets greedy rules
		check, err := bc.CheckGreedy(confirmBlock, block, uint64(bc.chainConfig.Greedy))
		if err != nil {
			return nil, err
		}
		if !check {
			return nil, ErrConfirmBlockNotMeetGreedy
		}

		confirmBlocks = append(confirmBlocks, confirmBlock)
		confirmedBlockSimpleHashSet.Add(confirmBlock.SimpleHash())
	}
	return confirmBlocks, nil
}
