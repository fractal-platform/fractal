package params

import "math/big"

var (
	OriginalMiningRewardValue    = big.NewInt(5 * 1e9) // 5FRA
	OriginalConfirmRewardValue   = big.NewInt(5 * 1e8) // 0.5FRA
	OriginalConfirmedRewardValue = big.NewInt(5 * 1e8) // 0.5FRA

	TransactionFeeCoefficient = big.NewInt(10) // TransactionFee = GasUsed * GasPrice / TransactionFeeCoefficient
)

const (
	MiningAwardCycle              uint64 = 3002368 // Must be divisible by 4096
	MaxMiningRewardReductionTimes uint64 = 10
)
