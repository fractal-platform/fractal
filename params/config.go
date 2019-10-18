package params

var (
	TargetGasLimit = GenesisGasLimit // The artificial target
)

const (
	GasLimitBoundDivisor uint64 = 1024 // The bound divisor of the gas limit, used in update calculations.
	MinGasLimit          uint64 = 1e10 // Minimum the gas limit may ever be.
	MaxGasLimit          uint64 = 4e10 // Maximum the gas limit may ever be.
	GenesisGasLimit      uint64 = 2e10 // Gas limit of the Genesis block.

	TxGas                   uint64 = 2000000 // Per transaction not creating a contract. NOTE: Not payable on data of calls between transactions.
	TxGasContractCreation   uint64 = 5000000 // Per transaction that creates a contract. NOTE: Not payable on data of calls between transactions.
	TxGasContractCreateData uint64 = 20000   //
	TxDataZeroGas           uint64 = 400     // Per byte of data attached to a transaction that equals zero. NOTE: Not payable on data of calls between transactions.
	TxDataNonZeroGas        uint64 = 6800    // Per byte of data attached to a transaction that is not equal to zero. NOTE: Not payable on data of calls between transactions.

	MaxCodeSize = 256 * 1024 * 1024 // Maximum bytecode to permit for a contract

	BloomByteSize               uint64 = 512
	BloomBitsSize                      = BloomByteSize * 8 // 4096, Must be divisible by 8
	ConfirmHeightDistance       uint64 = 6
	StakeRegisterHeightDistance uint64 = 6

	PackerKeyConfirmDistance uint64 = 6

	RoundsPerSecond = 10
)
