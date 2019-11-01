// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

import (
	"crypto/ecdsa"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/p2p/discover"
	"github.com/fractal-platform/fractal/utils/log"
)

const (
	DefaultRpcHost = "localhost" // Default host interface for the RPC server
	DefaultRpcPort = 8545        // Default TCP port for the RPC server
	DefaultDataDir = "data"      // Default data dir for the node

	datadirPrivateKey   = "nodekey"            // Path within the datadir to the node's private key
	datadirStaticNodes  = "static-nodes.json"  // Path within the datadir to the static node list
	datadirTrustedNodes = "trusted-nodes.json" // Path within the datadir to the trusted node list
	datadirNodeDatabase = "nodes"              // Path within the datadir to store the node infos
)

// NodeConfig represents the config set for the running node
type NodeConfig struct {
	// UserIdent, if set, is used as an additional component in the devp2p node identifier.
	UserIdent string `toml:",omitempty"`

	// Version should be set to the version number of the program. It is used
	// in the devp2p node identifier.
	Version string `toml:"-"`

	// DataDir is the file system folder the node should use for any data storage
	// requirements. The configured data directory will not be directly shared with
	// registered services, instead those can use utility methods to create/access
	// databases or flat files. This enables ephemeral nodes which can fully reside
	// in memory.
	DataDir string

	// Configuration of peer-to-peer networking.
	P2P p2p.Config

	// UseLightweightKDF lowers the memory and CPU requirements of the key store
	// scrypt KDF at the expense of security.
	UseLightweightKDF bool `toml:",omitempty"`

	// RpcEndpoint is the interface for the RPC server.
	RpcEndpoint string `toml:",omitempty"`

	//
	RpcApiList []string

	HTTPCors []string

	// Logger is a custom logger to use with the p2p.Server.
	Logger log.Logger `toml:",omitempty"`
}

// NewNodeConfig creates the default NodeConfig and return it
func NewNodeConfig() *NodeConfig {
	return &NodeConfig{
		DataDir: DefaultDataDir,
		P2P: p2p.Config{
			DiscListenAddr: ":30303",
			RwListenType:   uint8(1), //TCP
			RwListenAddr:   ":30303",
			MaxPeers:       25,
			NAT:            nil,
		},
	}
}

// NodeDB returns the path to the discovery node database.
func (c *NodeConfig) NodeDB() string {
	return c.ResolvePath(datadirNodeDatabase)
}

// NodeName returns the devp2p node identifier.
func (c *NodeConfig) NodeName() string {
	name := c.Progname()
	if c.UserIdent != "" {
		name += "/" + c.UserIdent
	}
	if c.Version != "" {
		name += "/v" + c.Version
	}
	name += "/" + runtime.GOOS + "-" + runtime.GOARCH
	name += "/" + runtime.Version()
	return name
}

func (c *NodeConfig) Progname() string {
	progname := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	if progname == "" {
		panic("empty executable name, set Config.Name")
	}
	return progname
}

// ResolvePath resolves path in the instance directory.
func (c *NodeConfig) ResolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.DataDir, path)
}

// NodeKey retrieves the currently configured private key of the node, checking
// first any manually set key, falling back to the one found in the configured
// data folder. If no key can be found, a new one is generated.
func (c *NodeConfig) NodeKey() *ecdsa.PrivateKey {
	// Use any specifically configured key.
	if c.P2P.PrivateKey != nil {
		return c.P2P.PrivateKey
	}

	keyfile := c.ResolvePath(datadirPrivateKey)
	if key, err := crypto.LoadECDSA(keyfile); err == nil {
		return key
	}

	// No persistent key found, generate and store a new one.
	key, err := crypto.GenerateKey()
	if err != nil {
		log.Crit(fmt.Sprintf("Failed to generate node key: %v", err))
	}
	if err := os.MkdirAll(c.DataDir, 0700); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
		return key
	}
	keyfile = filepath.Join(c.DataDir, datadirPrivateKey)
	if err := crypto.SaveECDSA(keyfile, key); err != nil {
		log.Error(fmt.Sprintf("Failed to persist node key: %v", err))
	}
	return key
}

// StaticNodes returns a list of node enode URLs configured as static nodes.
func (c *NodeConfig) StaticNodes() []*discover.Node {
	return c.parsePersistentNodes(c.ResolvePath(datadirStaticNodes))
}

// TrustedNodes returns a list of node enode URLs configured as trusted nodes.
func (c *NodeConfig) TrustedNodes() []*discover.Node {
	return c.parsePersistentNodes(c.ResolvePath(datadirTrustedNodes))
}

// parsePersistentNodes parses a list of discovery node URLs loaded from a .json
// file from within the data directory.
func (c *NodeConfig) parsePersistentNodes(path string) []*discover.Node {
	// Short circuit if no node config is present
	if c.DataDir == "" {
		return nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	// Load the nodes from the config file.
	var nodelist []string
	if err := common.LoadJSON(path, &nodelist); err != nil {
		log.Error(fmt.Sprintf("Can't load node file %s: %v", path, err))
		return nil
	}
	// Interpret the list as a discovery node array
	var nodes []*discover.Node
	for _, url := range nodelist {
		if url == "" {
			continue
		}
		node, err := discover.ParseNode(url)
		if err != nil {
			log.Error(fmt.Sprintf("Node URL %s: %v\n", url, err))
			continue
		}
		nodes = append(nodes, node)
	}
	return nodes
}
