// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package difficulty contains the implementation of difficulty change for fractal.

package difficulty

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/params"
	"math/big"
)

var (
	minimumDifficulty      = big.NewInt(1000 * 1000 * 1000) // The minimum that the difficulty may ever be.
	difficultyBoundDivisor = big.NewInt(2048)               // The bound divisor of the difficulty, used in the update calculations.
)

// round_diff = round - parent_round
// if [0,3] ,diff =parent_diff + parent_diff/2048*2
// if [4,7] ,diff =parent_diff + parent_diff/2048*1
// if [8,11] ,diff =parent_diff
// all follow the formula below
// diff = (parent_diff +
//         ((parent_diff / 2048) * max(2 - (round - parent_round) // 4, -99))
//        )
func CalcDifficulty(round uint64, parentRound uint64, parentDifficulty *big.Int) *big.Int {

	if parentDifficulty.Cmp(big.NewInt(0)) < 0 {
		return minimumDifficulty
	}
	if round <= parentRound {
		return parentDifficulty
	}
	bigRound := new(big.Int).SetUint64(round / params.RoundsPerSecond)
	bigParentRound := new(big.Int).SetUint64(parentRound / params.RoundsPerSecond)

	// holds intermediate values to make the fractal-diff easier to read & audit
	x := new(big.Int)
	y := new(big.Int)

	// 2 - (round - parent_round) // 4
	x.Sub(bigRound, bigParentRound)
	x.Div(x, common.Big4)
	x.Sub(common.Big2, x)

	// max(2 - (round - parent_round) // 4, -99)
	if x.Cmp(common.BigMinus99) < 0 {
		x.Set(common.BigMinus99)
	}
	// (parent_diff + parent_diff // 2048 * max(2 - (round - parent_round) // 4, -99))
	y.Div(parentDifficulty, difficultyBoundDivisor)
	x.Mul(y, x)
	x.Add(parentDifficulty, x)

	// minimum difficulty can ever be
	if x.Cmp(minimumDifficulty) < 0 {
		x.Set(minimumDifficulty)
	}
	return x
}
