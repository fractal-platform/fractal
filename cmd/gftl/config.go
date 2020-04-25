package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"unicode"

	"github.com/fractal-platform/fractal/cmd/utils"
	"github.com/fractal-platform/fractal/common/fdlimit"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/p2p/discover"
	"github.com/fractal-platform/fractal/p2p/nat"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/naoina/toml"
	"gopkg.in/urfave/cli.v1"
)

// These settings ensure that TOML keys use the same names as Go struct fields.
var tomlSettings = toml.Config{
	NormFieldName: func(rt reflect.Type, key string) string {
		return key
	},
	FieldToKey: func(rt reflect.Type, field string) string {
		return field
	},
	MissingField: func(rt reflect.Type, field string) error {
		link := ""
		if unicode.IsUpper(rune(rt.Name()[0])) && rt.PkgPath() != "main" {
			link = fmt.Sprintf(", see https://godoc.org/%s#%s for available fields", rt.PkgPath(), rt.Name())
		}
		return fmt.Errorf("field '%s' is not defined in %s%s", field, rt.String(), link)
	},
}

func loadConfig(file string, cfg *config.Config) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	err = tomlSettings.NewDecoder(bufio.NewReader(f)).Decode(cfg)
	// Add file name to errors that have a line number.
	if _, ok := err.(*toml.LineError); ok {
		err = errors.New(file + ", " + err.Error())
	}
	return err
}

// makeDatabaseHandles raises out the number of allowed file handles per process
// for gftl and returns half of the allowance to assign to the database.
func makeDatabaseHandles() int {
	limit, err := fdlimit.Current()
	if err != nil {
		utils.Fatalf("Failed to retrieve file descriptor allowance: %v", err)
	}
	if limit < 2048 {
		if err := fdlimit.Raise(2048); err != nil {
			utils.Fatalf("Failed to raise file descriptor allowance: %v", err)
		}
	}
	if limit > 2048 { // cap database file descriptors even if more is available
		limit = 2048
	}
	return limit / 2 // Leave half for networking and other stuff
}

func makeConfigNode(ctx *cli.Context) *config.Config {
	var cfg *config.Config

	//
	if ctx.GlobalBool(testnetFlag.Name) {
		cfg = &config.DefaultTestnetConfig
	} else if ctx.GlobalBool(testnet2Flag.Name) {
		cfg = &config.DefaultTestnet2Config
	} else if ctx.GlobalBool(testnet3Flag.Name) {
		cfg = &config.DefaultTestnet3Config
	} else if ctx.GlobalIsSet(configFileFlag.Name) {
		cfg = &config.DefaultConfig

		// Load config file.
		if file := ctx.GlobalString(configFileFlag.Name); file != "" {
			if err := loadConfig(file, cfg); err != nil {
				utils.Fatalf("%v", err)
			}
		}
	} else {
		cfg = &config.DefaultMainnetConfig
	}

	if ctx.GlobalBool(miningEnabledFlag.Name) {
		cfg.MinerEnable = true
	}

	if ctx.GlobalBool(packEnabledFlag.Name) {
		cfg.PackerEnable = true
	}
	if ctx.GlobalIsSet(packerIdFlag.Name) {
		cfg.PackerId = uint32(ctx.GlobalUint(packerIdFlag.Name))
	}

	cfg.DatabaseHandles = makeDatabaseHandles()

	// whether test fastSync network
	if ctx.GlobalBool(syncTestFlag.Name) {
		cfg.SyncTest = true
	}
	// load checkPoints file
	if checkPointFile := ctx.GlobalString(checkPointsFlag.Name); checkPointFile != "" && cfg.ChainConfig.CheckPointEnable {
		if data, err := ioutil.ReadFile(checkPointFile); err == nil {
			json.Unmarshal(data, &cfg.CheckPoints)
			log.Info("load checkPoints file success", "fileName", checkPointFile, "checkPoints", cfg.CheckPoints)
		}
	}

	// set node config
	cfg.NodeConfig = config.NewNodeConfig()
	cfg.NodeConfig.Version = params.VersionFull(versionMeta, gitCommit)
	setNodeConfig(ctx, cfg.NodeConfig)

	// load genesis alloc file
	if genesisAllocFile := ctx.GlobalString(genesisAllocFlag.Name); genesisAllocFile != "" {
		if data, err := ioutil.ReadFile(genesisAllocFile); err == nil {
			log.Info("load genesis alloc file success", "fileName", genesisAllocFile)
			unmarshalErr := json.Unmarshal(data, &cfg.Genesis.Alloc)
			if unmarshalErr != nil {
				log.Error("Unmarshal genesis alloc file error", "err", unmarshalErr)
			}
		}
	}

	// Unlock the miner key.
	pwd := ctx.GlobalString(unlockedAccountFlag.Name)
	if pwd != "" {
		cfg.KeyPass = pwd
	}
	cfg.MinerKeyFolder = cfg.NodeConfig.ResolvePath("keys/mining_keys/")
	cfg.PackerKeyFolder = cfg.NodeConfig.ResolvePath("keys/packer_keys/")

	return cfg
}

// setNodeUserIdent creates the user identifier from CLI flags.
func setNodeUserIdent(ctx *cli.Context, cfg *config.NodeConfig) {
	if identity := ctx.GlobalString(identityFlag.Name); len(identity) > 0 {
		cfg.UserIdent = identity
	}
}

// setListenAddress creates a TCP listening address string from set command
// line flags.
func setListenAddress(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(listenPortFlag.Name) {
		cfg.DiscListenAddr = fmt.Sprintf(":%d", ctx.GlobalInt(listenPortFlag.Name))
		cfg.RwListenType = uint8(1)
		cfg.RwListenAddr = fmt.Sprintf(":%d", ctx.GlobalInt(listenPortFlag.Name))
	}
}

// setBootstrapNodes creates a list of bootstrap nodes from the command line
// flags, reverting to pre-configured ones if none have been specified.
func setBootstrapNodes(ctx *cli.Context, cfg *p2p.Config) {
	cfg.BootstrapNodes = make([]*discover.Node, 0)
	if ctx.GlobalIsSet(bootnodesFlag.Name) {
		urls := strings.Split(ctx.GlobalString(bootnodesFlag.Name), ",")

		for _, url := range urls {
			node, err := discover.ParseNode(url)
			if err != nil {
				log.Crit("Bootstrap URL invalid", "enode", url, "err", err)
			}
			cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
		}
	}

	if ctx.GlobalIsSet(testnetFlag.Name) {
		node, _ := discover.ParseNode("enode://5b736302b16b83e5ae102de228ffd376b4cb4748a136057ea84bbbd6d1026a18aa902168af2d15f47ff9c300414bf6999f6525f3b18e9225afb70c7b35dd22ed@161.189.2.180:60002")
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	} else if ctx.GlobalIsSet(testnet2Flag.Name) {
		node, _ := discover.ParseNode("enode://de08d9a677afac2aa46b1307f4e8ab3f8d04ffcf428f50d3084199403c358087a3ba49ecb33a99ae9908c406a975d1e42fe26a38a73359cfaf0f5802e267bc32@52.83.78.168:60003")
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	} else if ctx.GlobalIsSet(testnet3Flag.Name) {
		node, _ := discover.ParseNode("enode://c063c4e9fcd190328a7f010d9e6474c8588c2175c73c277091d5ff3cc5a136c849299817fecb2bfb33e273c10c77d457c513436a8209d7623e62b83fa9b1c7ab@210.22.171.162:50004")
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node)
	} else {
		// mainnet
		node0, _ := discover.ParseNode("enode://e17397cc0b08a4bf92dfc0ea12ce370d22082ce26c823242b00495d879b82aee606e0e9dcb9739695a0e8b5f4eb57c3e71ba58b86a13a6e3926bf41eb285d069@3.226.27.211:60006")
		node1, _ := discover.ParseNode("enode://50ae85e040f6b7ba0f61d31fe7f242602bb85a65b5bd5a3801470c3849f056ff31bf34b66dcef3ef60ddebf92831de7ec8f5bc7b6764136ac110ee6313a1933c@34.196.83.105:60006")
		node2, _ := discover.ParseNode("enode://a441c0048813a0ee6927752de8fde19a410b04671ac866c75a96f02a3ed91e8f8955c0f22004d5781f2e7fd9d9ce0a35be33495ecd5b2cf918a73146d906a105@35.170.127.58:60006")
		node3, _ := discover.ParseNode("enode://53ac135a6392f4e03ddaf530ba697d179912683394840642c5b74f66cd9d02100d322b16b17d8bc1b15bef1a57a664e10d765030b90bae8121257442f0b4872a@52.55.182.221:60006")
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node0)
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node1)
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node2)
		cfg.BootstrapNodes = append(cfg.BootstrapNodes, node3)
	}
}

// setNAT creates a port mapper from command line flags.
func setNAT(ctx *cli.Context, cfg *p2p.Config) {
	if ctx.GlobalIsSet(natFlag.Name) {
		natif, err := nat.Parse(ctx.GlobalString(natFlag.Name))
		if err != nil {
			utils.Fatalf("Option %s: %v", natFlag.Name, err)
		}
		cfg.NAT = natif
	}
}

// splitAndTrim splits input separated by a comma
// and trims excessive white space from the substrings.
func splitAndTrim(input string) []string {
	result := strings.Split(input, ",")
	for i, r := range result {
		result[i] = strings.TrimSpace(r)
	}
	return result
}

// setRpc creates the HTTP RPC listener interface string from the set
// command line flags, returning empty if the HTTP endpoint is disabled.
func setRpc(ctx *cli.Context, cfg *config.NodeConfig) {
	if ctx.GlobalBool(rpcEnabledFlag.Name) {
		rpcPort := 8545
		if ctx.GlobalIsSet(rpcPortFlag.Name) {
			rpcPort = ctx.GlobalInt(rpcPortFlag.Name)
		}

		rpcHost := "127.0.0.1"
		if ctx.GlobalIsSet(rpcListenAddrFlag.Name) {
			rpcHost = ctx.GlobalString(rpcListenAddrFlag.Name)
		}

		if ctx.GlobalIsSet(rpcCORSDomainFlag.Name) {
			cfg.HTTPCors = splitAndTrim(ctx.GlobalString(rpcCORSDomainFlag.Name))
		}

		cfg.RpcEndpoint = fmt.Sprintf("%s:%d", rpcHost, rpcPort)

		//
		cfg.RpcApiList = ctx.GlobalStringSlice(rpcApiFlag.Name)
	}
}

func setP2PConfig(ctx *cli.Context, cfg *p2p.Config) {
	setNAT(ctx, cfg)
	setListenAddress(ctx, cfg)
	setBootstrapNodes(ctx, cfg)

	log.Info("Maximum peer count", "total", cfg.MaxPeers)

	if ctx.GlobalIsSet(maxPendingPeersFlag.Name) {
		cfg.MaxPendingPeers = ctx.GlobalInt(maxPendingPeersFlag.Name)
	}
	if ctx.GlobalIsSet(noDiscoverFlag.Name) {
		cfg.NoDiscovery = true
	}
}

// SetNodeConfig applies node-related command line flags to the config.
func setNodeConfig(ctx *cli.Context, cfg *config.NodeConfig) {
	setP2PConfig(ctx, &cfg.P2P)
	setRpc(ctx, cfg)
	setNodeUserIdent(ctx, cfg)

	if ctx.GlobalIsSet(dataDirFlag.Name) {
		cfg.DataDir = ctx.GlobalString(dataDirFlag.Name)
	} else {
		if ctx.GlobalBool(testnetFlag.Name) {
			cfg.DataDir = filepath.Join(config.DefaultDataDir, "testnet")
		} else if ctx.GlobalBool(testnet2Flag.Name) {
			cfg.DataDir = filepath.Join(config.DefaultDataDir, "testnet2")
		} else if ctx.GlobalBool(testnet3Flag.Name) {
			cfg.DataDir = filepath.Join(config.DefaultDataDir, "testnet3")
		} else {
			cfg.DataDir = config.DefaultDataDir
		}
	}

	if cfg.DataDir != "" {
		cfg.DataDir, _ = filepath.Abs(cfg.DataDir)
	}

	log.Info("Node data dir", "path", cfg.DataDir)
}
