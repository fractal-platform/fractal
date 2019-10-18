// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
)

// FractalAPI provides an API to access Fractal related information.
// It offers only methods that operate on public data that is freely available to anyone.
type FractalAPI struct {
	ftl fractal
}

// NewFractalAPI creates a new Fractal protocol API.
func NewFractalAPI(ftl fractal) *FractalAPI {
	return &FractalAPI{ftl}
}

// Coinbase is the address that mining rewards will be send to
func (s *FractalAPI) Coinbase() common.Address {
	coinbase := s.ftl.Coinbase()
	return coinbase
}

// ProtocolVersion returns the current Fractal protocol version this node supports
func (s *FractalAPI) ProtocolVersion() hexutil.Uint {
	return hexutil.Uint(s.ftl.FtlVersion())
}

func (s *FractalAPI) ChainId() hexutil.Uint64 {
	return hexutil.Uint64(s.ftl.Config().ChainConfig.ChainID)
}
