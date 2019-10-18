// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"errors"
	"fmt"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/p2p/discover"
)

var ErrNodeStopped = errors.New("node not started")

// AdminAPI is the collection of administrative API methods.
type AdminAPI struct {
	server *p2p.Server
	ftl    fractal
}

// NewAdminAPI creates a new AdminAPI.
func NewAdminAPI(server *p2p.Server, ftl fractal) *AdminAPI {
	return &AdminAPI{server, ftl}
}

// AddPeer requests connecting to a remote node, and also maintaining the new
// connection at all times, even reconnecting if it is lost.
func (api *AdminAPI) AddPeer(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	// Try to add the url as a static peer and return
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.AddPeer(node)
	return true, nil
}

// RemovePeer disconnects from a remote node if the connection exists
func (api *AdminAPI) RemovePeer(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	// Try to remove the url as a static peer and return
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.RemovePeer(node)
	return true, nil
}

// AddTrustedPeer allows a remote node to always connect, even if slots are full
func (api *AdminAPI) AddTrustedPeer(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.AddTrustedPeer(node)
	return true, nil
}

// RemoveTrustedPeer removes a remote node from the trusted peer set, but it
// does not disconnect it automatically.
func (api *AdminAPI) RemoveTrustedPeer(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.RemoveTrustedPeer(node)
	return true, nil
}

//
func (api *AdminAPI) AddBlack(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	// Try to add the url as a static peer and return
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.AddBlack(node)
	return true, nil
}

//
func (api *AdminAPI) RemoveBlack(url string) (bool, error) {
	// Make sure the server is running, fail otherwise
	server := api.server
	if server == nil {
		return false, ErrNodeStopped
	}
	// Try to remove the url as a static peer and return
	node, err := discover.ParseNode(url)
	if err != nil {
		return false, fmt.Errorf("invalid enode: %v", err)
	}
	server.RemoveBlack(node)
	return true, nil
}

// Start the miner
func (api *AdminAPI) StartMining() error {
	// Start the miner and return
	if !api.ftl.IsMining() {
		return api.ftl.StartMining()
	}
	return nil
}

// Stop the miner
func (api *AdminAPI) StopMining() {
	api.ftl.StopMining()
}

// Mining returns an indication if this node is currently mining.
func (api *AdminAPI) Mining() bool {
	return api.ftl.IsMining()
}

//
func (api *AdminAPI) GenerateMiningKey(address common.Address) keys.MiningPubkey {
	pubkey := api.ftl.MiningKeyManager().CreateKey(address)
	return keys.PublicKeyForMining(pubkey)
}

func (api *AdminAPI) StartPacking(packerIndex uint32) {
	api.ftl.Packer().StartPacking(packerIndex)
}

func (api *AdminAPI) StopPacking() {
	api.ftl.Packer().StopPacking()
}

func (api *AdminAPI) IsPacking() bool {
	return api.ftl.Packer().IsPacking()
}
