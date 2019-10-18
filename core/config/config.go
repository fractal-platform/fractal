// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

// DefaultConfig contains default settings for use on the Fractal private net.
var DefaultConfig = Config{
	FakeMode: false,

	DatabaseCache: 768,

	PkgCacheSize:        1024,
	PackerInfoCacheSize: 16,

	PackerEnable:      false,
	PackerInterval:    1,
	PackerCollectAddr: ":8899",

	MinerEnable:   false,
	TxPoolConfig:  &DefaultPoolConfig,
	PkgPoolConfig: &DefaultPoolConfig,
	SyncConfig:    &DefaultSyncConfig,
	SyncTest:      false,
}

// DefaultMainnetConfig contains default settings for use on the Fractal main net.
var DefaultMainnetConfig = Config{
	FakeMode: false,

	DatabaseCache: 768,

	ChainConfig: MainnetChainConfig,
	Genesis:     DefaultMainnetGenesisBlock(),

	PkgCacheSize:        1024,
	PackerInfoCacheSize: 16,

	PackerEnable:      false,
	PackerInterval:    1,
	PackerCollectAddr: ":8899",

	MinerEnable:   false,
	TxPoolConfig:  &DefaultPoolConfig,
	PkgPoolConfig: &DefaultPoolConfig,
	SyncConfig:    &DefaultSyncConfig,
	SyncTest:      false,
}

// DefaultConfig contains default settings for use on the Fractal test net.
var DefaultTestnetConfig = Config{
	FakeMode: false,

	DatabaseCache: 768,

	ChainConfig: TestnetChainConfig,
	Genesis:     DefaultTestnetGenesisBlock(),

	PkgCacheSize:        1024,
	PackerInfoCacheSize: 16,

	PackerEnable:      false,
	PackerInterval:    1,
	PackerCollectAddr: ":9899",

	MinerEnable:   false,
	TxPoolConfig:  &DefaultPoolConfig,
	PkgPoolConfig: &DefaultPoolConfig,
	SyncConfig:    &DefaultSyncConfig,
	SyncTest:      false,
}

// DefaultConfig contains default settings for use on the Fractal test net.
var DefaultTestnet2Config = Config{
	FakeMode: false,

	DatabaseCache: 768,

	ChainConfig: Testnet2ChainConfig,
	Genesis:     DefaultTestnet2GenesisBlock(),

	PkgCacheSize:        1024,
	PackerInfoCacheSize: 16,

	PackerEnable:      false,
	PackerInterval:    1,
	PackerCollectAddr: ":9899",

	MinerEnable:   false,
	TxPoolConfig:  &DefaultPoolConfig,
	PkgPoolConfig: &DefaultPoolConfig,
	SyncConfig:    &DefaultSyncConfig,
	SyncTest:      false,
}

// DefaultConfig contains default settings for use on the Fractal test net.
var DefaultTestnet3Config = Config{
	FakeMode: false,

	DatabaseCache: 768,

	ChainConfig: Testnet3ChainConfig,
	Genesis:     DefaultTestnet3GenesisBlock(),

	PkgCacheSize:        1024,
	PackerInfoCacheSize: 16,

	PackerEnable:      false,
	PackerInterval:    2,
	PackerCollectAddr: ":9899",

	MinerEnable:   false,
	TxPoolConfig:  &DefaultPoolConfig,
	PkgPoolConfig: &DefaultPoolConfig,
	SyncConfig:    &DefaultSyncConfig,
	SyncTest:      false,
}

type Config struct {
	FakeMode bool

	// Database options
	DatabaseHandles int `toml:"-"`
	DatabaseCache   int `toml:"-"`

	//
	NodeConfig *NodeConfig `toml:",omitempty"`

	// The chain config
	// If nil, the Fractal mainnet config will be used.
	ChainConfig *ChainConfig `toml:",omitempty"`

	// sync config
	// when there is need to do fast sync
	SyncConfig *SyncConfig

	Genesis *Genesis

	KeyPass string

	PkgCacheSize        int
	PackerInfoCacheSize uint8

	PackerEnable      bool
	PackerId          uint32
	PackerInterval    int
	PackerCollectAddr string
	PackerKeyFolder   string

	TxBatchSendToPackInterval int
	TxSaveProcessInterval     int

	MinerEnable    bool
	TxPoolConfig   *PoolConfig `toml:",omitempty"`
	PkgPoolConfig  *PoolConfig `toml:",omitempty"`
	MinerKeyFolder string

	CheckPoints *CheckPoints // checkPoints

	SyncTest bool
}
