package main

import (
	"github.com/fractal-platform/fractal/cmd/utils"
	"github.com/fractal-platform/fractal/core/config"
	"gopkg.in/urfave/cli.v1"
)

// These are all the command line flags we support.
// If you add to this list, please remember to include the
// flag in the appropriate command definition.
//
// The flags are defined here so their names and help texts
// are the same for all commands.
var (
	// file config
	configFileFlag = cli.StringFlag{
		Name:  "config",
		Usage: "TOML configuration file",
	}
	genesisAllocFlag = cli.StringFlag{
		Name:  "genesisAlloc",
		Usage: "genesis_alloc configuration file",
	}
	checkPointsFlag = cli.StringFlag{
		Name:  "checkPoint",
		Usage: "checkPoints configuration file",
	}

	// General settings
	dataDirFlag = utils.DirectoryFlag{
		Name:  "datadir",
		Usage: "Data directory for the databases and keys",
		Value: utils.DirectoryString{Value: "data"},
	}
	testnetFlag = cli.BoolFlag{
		Name:  "testnet",
		Usage: "Test network: pre-configured test network",
	}
	testnet2Flag = cli.BoolFlag{
		Name:  "testnet2",
		Usage: "Test network: pre-configured test2 network",
	}
	testnet3Flag = cli.BoolFlag{
		Name:  "testnet3",
		Usage: "Test network: pre-configured test3 network",
	}
	syncTestFlag = cli.BoolFlag{
		Name:  "synctest",
		Usage: "test fastsync pre-configured test fastsync",
	}
	generalFlags = []cli.Flag{
		dataDirFlag,
		testnetFlag,
		testnet2Flag,
		testnet3Flag,
		syncTestFlag,
	}

	// Miner settings
	miningEnabledFlag = cli.BoolFlag{
		Name:  "mine",
		Usage: "Enable mining",
	}

	// Packer settings
	packEnabledFlag = cli.BoolFlag{
		Name:  "packer",
		Usage: "Enable packer",
	}
	packerIdFlag = cli.UintFlag{
		Name:  "packerId",
		Usage: "Set packer index",
	}

	// Account settings
	unlockedAccountFlag = cli.StringFlag{
		Name:  "unlock",
		Usage: "The password to use for unlock the miner's private key",
		Value: "",
	}

	// RPC settings
	rpcEnabledFlag = cli.BoolFlag{
		Name:  "rpc",
		Usage: "Enable the HTTP-RPC server",
	}
	rpcListenAddrFlag = cli.StringFlag{
		Name:  "rpcaddr",
		Usage: "HTTP-RPC server listening interface",
		Value: config.DefaultRpcHost,
	}
	rpcPortFlag = cli.IntFlag{
		Name:  "rpcport",
		Usage: "HTTP-RPC server listening port",
		Value: config.DefaultRpcPort,
	}
	rpcApiFlag = cli.StringSliceFlag{
		Name:  "rpcapi",
		Usage: "HTTP-RPC server api list",
		Value: nil,
	}
	rpcCORSDomainFlag = cli.StringFlag{
		Name:  "rpccorsdomain",
		Usage: "Comma separated list of domains from which to accept cross origin requests (browser enforced)",
		Value: "",
	}
	rpcFlags = []cli.Flag{
		rpcEnabledFlag,
		rpcListenAddrFlag,
		rpcPortFlag,
		rpcApiFlag,
		rpcCORSDomainFlag,
	}

	// Network Settings
	identityFlag = cli.StringFlag{
		Name:  "identity",
		Usage: "Custom node name",
	}
	maxPeersFlag = cli.IntFlag{
		Name:  "maxpeers",
		Usage: "Maximum number of network peers (network disabled if set to 0)",
		Value: 25,
	}
	maxPendingPeersFlag = cli.IntFlag{
		Name:  "maxpendpeers",
		Usage: "Maximum number of pending connection attempts (defaults used if set to 0)",
		Value: 0,
	}
	listenPortFlag = cli.IntFlag{
		Name:  "port",
		Usage: "Network listening port",
		Value: 30303,
	}
	bootnodesFlag = cli.StringFlag{
		Name:  "bootnodes",
		Usage: "Comma separated enode URLs for P2P discovery bootstrap",
		Value: "",
	}
	natFlag = cli.StringFlag{
		Name:  "nat",
		Usage: "NAT port mapping mechanism (any|none|upnp|pmp|extip:<IP>)",
		Value: "none",
	}
	noDiscoverFlag = cli.BoolFlag{
		Name:  "nodiscover",
		Usage: "Disables the peer discovery mechanism (manual peer addition)",
	}
	networkFlags = []cli.Flag{
		identityFlag,
		maxPeersFlag,
		maxPendingPeersFlag,
		listenPortFlag,
		bootnodesFlag,
		natFlag,
		noDiscoverFlag,
	}

	// Metrics flags
	metricsEnabledFlag = cli.BoolFlag{
		Name:  "metrics",
		Usage: "Enable metrics collection and reporting",
	}
	influxdbUrlFlag = cli.StringFlag{
		Name:  "influxdburl",
		Usage: "Influxdb url for metrics",
	}
	influxdbDatabaseFlag = cli.StringFlag{
		Name:  "influxdbdatabase",
		Usage: "Influxdb database for metrics",
	}
	influxdbUsernameFlag = cli.StringFlag{
		Name:  "influxdbusername",
		Usage: "Influxdb username for metrics",
	}
	influxdbPasswordFlag = cli.StringFlag{
		Name:  "influxdbpassword",
		Usage: "Influxdb password for metrics",
	}
	metricsFlags = []cli.Flag{
		metricsEnabledFlag,
		influxdbUrlFlag,
		influxdbDatabaseFlag,
		influxdbUsernameFlag,
		influxdbPasswordFlag,
	}

	// Debug flags
	verbosityFlag = cli.IntFlag{
		Name:  "verbosity",
		Usage: "Logging verbosity: 0=silent, 1=error, 2=warn, 3=info, 4=debug",
		Value: 3,
	}
	pprofFlag = cli.BoolFlag{
		Name:  "pprof",
		Usage: "Enable the pprof HTTP server",
	}
	pprofPortFlag = cli.IntFlag{
		Name:  "pprofport",
		Usage: "pprof HTTP server listening port",
		Value: 6060,
	}
	pprofAddrFlag = cli.StringFlag{
		Name:  "pprofaddr",
		Usage: "pprof HTTP server listening interface",
		Value: "127.0.0.1",
	}
	debugFlags = []cli.Flag{
		verbosityFlag,
		pprofFlag,
		pprofPortFlag,
		pprofAddrFlag,
	}
)
