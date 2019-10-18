// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcserver contains implementations for net rpc server.
package rpcserver

// RpcApi describes the set of methods offered over the RPC interface
type RpcApi struct {
	Namespace string      // namespace under which the rpc methods of Service are exposed
	Version   string      // api version
	Service   interface{} // receiver instance which holds the methods
}
