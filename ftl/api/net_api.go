// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"context"
	"fmt"

	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/rpc/server"
)

// NetAPI offers network related RPC methods
type NetAPI struct {
	server         *p2p.Server
	networkVersion uint64
}

// NewNetAPI creates a new net API instance.
func NewNetAPI(server *p2p.Server, networkVersion uint64) *NetAPI {
	return &NetAPI{server, networkVersion}
}

// Listening returns an indication if the node is listening for network connections.
func (s *NetAPI) Listening() bool {
	return true // always listening
}

// PeerCount returns the number of connected peers
func (s *NetAPI) PeerCount() (hexutil.Uint, error) {
	server := s.server
	if server == nil {
		return 0, ErrNodeStopped
	}
	return hexutil.Uint(s.server.PeerCount()), nil
}

// Peers retrieves all the information we know about each individual peer at the
// protocol granularity.
func (s *NetAPI) Peers() ([]*p2p.PeerInfo, error) {
	server := s.server
	if server == nil {
		return nil, ErrNodeStopped
	}
	return server.PeersInfo(), nil
}

// PeerEvents creates an RPC subscription which receives peer events from the
// node's p2p.Server
func (s *NetAPI) PeerEvents(ctx context.Context) (*rpcserver.Subscription, error) {
	// Make sure the server is running, fail otherwise
	server := s.server
	if server == nil {
		return nil, ErrNodeStopped
	}

	// Create the subscription
	notifier, supported := rpcserver.NotifierFromContext(ctx)
	if !supported {
		return nil, rpc.ErrNotificationsUnsupported
	}
	rpcSub := notifier.CreateSubscription()

	go func() {
		events := make(chan *p2p.PeerEvent)
		sub := server.SubscribeEvents(events)
		defer sub.Unsubscribe()

		for {
			select {
			case event := <-events:
				notifier.Notify(rpcSub.ID, event)
			case <-sub.Err():
				return
			case <-rpcSub.Err():
				return
			case <-notifier.Closed():
				return
			}
		}
	}()

	return rpcSub, nil
}

// NodeInfo retrieves all the information we know about the host node at the
// protocol granularity.
func (s *NetAPI) NodeInfo() (*p2p.NodeInfo, error) {
	server := s.server
	if server == nil {
		return nil, ErrNodeStopped
	}
	return server.NodeInfo(), nil
}

// Version returns the current ftl protocol version.
func (s *NetAPI) Version() string {
	return fmt.Sprintf("%d", s.networkVersion)
}
