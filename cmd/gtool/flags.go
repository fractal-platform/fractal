package main

import "gopkg.in/urfave/cli.v1"

var (
	VerbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "verbosity for log",
		Value: 3,
	}
	ChainIdFlag = cli.IntFlag{
		Name:  "chainid",
		Usage: "chain id",
	}

	// for keys
	KeyFolderFlag = cli.StringFlag{
		Name:  "keys",
		Usage: "The Folder for all the key files",
		Value: "",
	}
	PasswordFlag = cli.StringFlag{
		Name:  "pass",
		Usage: "The password for keys",
		Value: "",
	}
	AddressFlag = cli.StringFlag{
		Name:  "addr",
		Usage: "The address for keys",
		Value: "",
	}

	// for genesis param
	GenesisStakeFlag = cli.Uint64Flag{
		Name:  "gstake",
		Usage: "The total stake in genesis state",
		Value: 1e17,
	}
	PackerKeyContractOwnerFlag = cli.StringFlag{
		Name:  "packerKeyOwner",
		Usage: "The owner address of packer key contract stake",
	}

	// for rpc
	RpcFlag = cli.StringFlag{
		Name:  "rpc",
		Usage: "rpc service address",
	}
	PackerFlag = cli.BoolFlag{
		Name:  "packer",
		Usage: "whether rpc server is packer or not",
	}

	// for tx
	ToFlag = cli.StringFlag{
		Name:  "to",
		Usage: "to address",
	}
	ValueFlag = cli.Int64Flag{
		Name:  "value",
		Usage: "transfer value",
		Value: 1,
	}

	// for batch tx
	TpsFlag = cli.IntFlag{
		Name:  "tps",
		Usage: "tps for current test",
	}
	NProcessFlag = cli.IntFlag{
		Name:  "nprocess",
		Usage: "process count",
	}

	// for contract
	WasmFlag = cli.StringFlag{
		Name:  "wasm",
		Usage: "wasm file path",
	}
	AbiFlag = cli.StringFlag{
		Name:  "abi",
		Usage: "abi file path",
	}
	ActionFlag = cli.StringFlag{
		Name:  "action",
		Usage: "action name",
	}
	ArgsFlag = cli.StringFlag{
		Name:  "args",
		Usage: "args json",
	}

	// for storage
	TableFlag = cli.StringFlag{
		Name:  "table",
		Usage: "table name",
	}
	StorageKeyFlag = cli.StringFlag{
		Name:  "skey",
		Usage: "storage key",
	}

	// for block
	BlockHeightFlag = cli.Uint64Flag{
		Name:  "height",
		Usage: "block height",
	}
	BlockHashFlag = cli.StringFlag{
		Name:  "bhash",
		Usage: "block hash",
	}

	// for packer
	PackerIdFlag = cli.Uint64Flag{
		Name:  "packerId",
		Usage: "packer index",
	}
	PackerRpcAddressFlag = cli.StringFlag{
		Name:  "packerAddress",
		Usage: "packer rpc address",
	}
	PackerCoinbaseFlag = cli.StringFlag{
		Name:  "packerCoinbase",
		Usage: "packer coinbase",
	}
	PackerPubKeyFlag = cli.StringFlag{
		Name:  "packerPubKey",
		Usage: "packer public key (ECDSA)",
	}

	// for database
	StateRootHashFlag = cli.StringFlag{
		Name:  "rootHash",
		Usage: "the hash of the state",
	}
	DbPathFlag = cli.StringFlag{
		Name:  "dbPath",
		Usage: "the database path",
	}
	OutPutPathFlag = cli.StringFlag{
		Name:  "outputPath",
		Usage: "alloc json file",
	}
	Keccak256HashFlag = cli.StringFlag{
		Name:  "k256Hash",
		Usage: "Keccak256 hash result",
	}
)
