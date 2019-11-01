// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

import (
	"encoding/json"
	"errors"

	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	MainnetChainID     = uint64(1)
	MainnetChainConfig = &ChainConfig{ChainID: MainnetChainID, Greedy: 4, TxExecutorType: "wasm", TxSignerType: "eip155", BlockSigFake: false, MaxNonceBitLength: 1024, CheckPointEnable: false, PackerGroupSize: 1}

	TestnetChainID     = uint64(2)
	TestnetChainConfig = &ChainConfig{ChainID: TestnetChainID, Greedy: 4, TxExecutorType: "wasm", TxSignerType: "eip155", BlockSigFake: false, MaxNonceBitLength: 1024000, CheckPointEnable: false, PackerGroupSize: 1}

	Testnet2ChainID     = uint64(3)
	Testnet2ChainConfig = &ChainConfig{ChainID: Testnet2ChainID, Greedy: 4, TxExecutorType: "wasm", TxSignerType: "eip155", BlockSigFake: false, MaxNonceBitLength: 1024000, CheckPointEnable: false, PackerGroupSize: 1}

	Testnet3ChainID     = uint64(4)
	Testnet3ChainConfig = &ChainConfig{ChainID: Testnet3ChainID, Greedy: 4, TxExecutorType: "wasm", TxSignerType: "eip155", BlockSigFake: false, MaxNonceBitLength: 1024000, CheckPointEnable: false, PackerGroupSize: 1}

	ErrChainConfigConflict = errors.New("Input chain config conflicts with the stored chain config")
	ErrPackerGroupSize     = errors.New("Input chain config param:<PackerGroupSize> can't be 0")
)

// ChainConfig is the config for the current chain.
// ChainConfig will be stored in the database while initializing the chain.
type ChainConfig struct {
	ChainID           uint64 `json:"chainId"` // chainId identifies the current chain and is used for replay protection
	Greedy            uint8  `json:"greedy"`  //
	TxExecutorType    string `json:"txExecutorType"`
	TxSignerType      string `json:"txSignerType"`
	BlockSigFake      bool   `json:"blockSigFake"`
	MaxNonceBitLength uint64 `json:"maxNonceBitLength"`
	CheckPointEnable  bool   `json:"checkPointEnable"`
	PackerGroupSize   uint64 `json:"packerGroupSize"`
}

func readFromDatabase(db dbwrapper.Database) *ChainConfig {
	data, err := dbaccessor.ReadChainConfig(db)
	if err != nil || data == nil {
		//log.Error("Read chain from database failed", "err", err)
		return nil
	}

	var config ChainConfig
	if err := json.Unmarshal(data, &config); err != nil {
		log.Error("Invalid chain config JSON", "err", err)
		return nil
	}
	return &config
}

func writeToDatabase(db dbwrapper.Database, chainConfig *ChainConfig) {
	if chainConfig == nil {
		return
	}

	data, err := json.Marshal(chainConfig)
	if err != nil {
		log.Crit("Failed to JSON encode chain config", "err", err)
		return
	}

	err = dbaccessor.WriteChainConfig(db, data)
	if err != nil {
		log.Crit("Failed to store chain config", "err", err)
		return
	}
}

// SetupChainConfig determine the current chain config, and write chain config to db.
func SetupChainConfig(db dbwrapper.Database, config *ChainConfig) (*ChainConfig, error) {
	storedConfig := readFromDatabase(db)
	if storedConfig != nil {
		if config != nil {
			if *config == *storedConfig {
				// input config equals to stored config
				return config, nil
			} else {
				// input config not equals to stored config, cause error
				return nil, ErrChainConfigConflict
			}
		} else {
			return storedConfig, nil
		}
	} else {
		var newConfig *ChainConfig
		if config != nil && config.ChainID != 0 {
			if config.PackerGroupSize == 0 {
				return nil, ErrPackerGroupSize
			}

			newConfig = config
		} else {
			newConfig = MainnetChainConfig
		}
		writeToDatabase(db, newConfig)
		return newConfig, nil
	}
}
