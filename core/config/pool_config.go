// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package config contains the normal config for other modules.
package config

import (
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/utils/log"
)

type PoolConfig struct {
	Locals      []common.Address // Addresses that should be treated by default as local
	NoLocals    bool             // Whether local transaction handling should be disabled
	FakeMode    bool
	Journal     string        // Journal of local transactions to survive node restarts
	Rejournal   time.Duration // Time interval to regenerate the local transaction journal
	PriceLimit  uint64        // Minimum gas price to enforce for acceptance into the pool
	PriceBump   uint64        // Minimum price bump percentage to replace an already existing transaction (nonce)
	GlobalQueue uint64        // Maximum number of non-executable transaction slots for all accounts
	Lifetime    time.Duration // Maximum amount of time non-executable transaction are queued

	// fake
	StartCleanTime     int64
	CleanPeriod        int64
	LeftEleNumEachAddr int
}

// DefaultPoolConfig contains the default configurations for the transaction
// pool.
var DefaultPoolConfig = PoolConfig{
	NoLocals:    true,
	FakeMode:    false,
	Journal:     "elements.rlp",
	Rejournal:   time.Hour,
	PriceLimit:  1,
	PriceBump:   10,
	GlobalQueue: 4096,
	Lifetime:    3 * time.Hour,
}

// Sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *PoolConfig) Sanitize() PoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		log.Warn("Sanitizing invalid pool journal time", "provided", conf.Rejournal, "updated", time.Second)
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		log.Warn("Sanitizing invalid pool price limit", "provided", conf.PriceLimit, "updated", DefaultPoolConfig.PriceLimit)
		conf.PriceLimit = DefaultPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		log.Warn("Sanitizing invalid txpool price bump", "provided", conf.PriceBump, "updated", DefaultPoolConfig.PriceBump)
		conf.PriceBump = DefaultPoolConfig.PriceBump
	}
	return conf
}
