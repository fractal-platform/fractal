// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

import (
	"testing"

	"github.com/fractal-platform/fractal/dbwrapper"
	. "github.com/stretchr/testify/assert"
)

func TestSetupChainConfig(t *testing.T) {
	t.Run("config==nil && stored==nil", func(t *testing.T) {
		db := dbwrapper.NewMemDatabase()
		config, err := SetupChainConfig(db, nil)
		Equal(t, *config, *MainnetChainConfig)
		Nil(t, err)
	})

	t.Run("config!=nil && stored==nil", func(t *testing.T) {
		db := dbwrapper.NewMemDatabase()
		cfg := &ChainConfig{
			ChainID:         88,
			Greedy:          5,
			PackerGroupSize: 16,
		}
		config, err := SetupChainConfig(db, cfg)
		Equal(t, *config, *cfg)
		Nil(t, err)
	})

	t.Run("config==nil && stored!=nil", func(t *testing.T) {
		db := dbwrapper.NewMemDatabase()
		oldCfg := &ChainConfig{
			ChainID:         88,
			Greedy:          5,
			PackerGroupSize: 16,
		}
		SetupChainConfig(db, oldCfg)

		config, err := SetupChainConfig(db, nil)
		Equal(t, *config, *oldCfg)
		Nil(t, err)
	})

	t.Run("config!=nil && stored!=nil", func(t *testing.T) {
		db := dbwrapper.NewMemDatabase()
		oldCfg := &ChainConfig{
			ChainID:         88,
			Greedy:          5,
			PackerGroupSize: 16,
		}
		SetupChainConfig(db, oldCfg)

		newCfg := &ChainConfig{
			ChainID:         88,
			Greedy:          5,
			PackerGroupSize: 16,
		}
		config, err := SetupChainConfig(db, newCfg)
		Equal(t, *config, *newCfg)
		Nil(t, err)

		newCfg = &ChainConfig{
			ChainID:         77,
			Greedy:          5,
			PackerGroupSize: 16,
		}
		config, err = SetupChainConfig(db, newCfg)
		Nil(t, config)
		NotNil(t, err)
	})
}
