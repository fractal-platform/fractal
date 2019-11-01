// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package p2p_handler contains the implementation of p2p handler for fractal.
package network

import (
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/p2p"
)

// peers represents the collection of active peers
type Peers struct {
	peers  map[string]*Peer
	lock   sync.RWMutex
	closed bool
}

// Register injects a new peer into the working set, or returns an error if the
// peer is already known. If a new peer it registered, its broadcast loop is also
// started.
func (ps *Peers) Register(p *Peer) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	if ps.closed {
		return errClosed
	}
	if _, ok := ps.peers[p.GetID()]; ok {
		return errAlreadyRegistered
	}
	ps.peers[p.GetID()] = p
	go p.loop()

	return nil
}

func (ps *Peers) GetPeers() map[string]*Peer {
	return ps.peers
}

// Unregister removes a remote peer from the active set, disabling any further
// actions to/from that particular entity.
func (ps *Peers) Unregister(id string) error {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	p, ok := ps.peers[id]
	if !ok {
		return errNotRegistered
	}
	delete(ps.peers, id)
	p.close()

	return nil
}

// Peer retrieves the registered peer with the given id.
func (ps *Peers) Peer(id string) *Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return ps.peers[id]
}

// Len returns if the current number of peers in the set.
func (ps *Peers) Len() int {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	return len(ps.peers)
}

// PeersWithoutBlock retrieves a list of peers that do not have a given block in
// their set of known hashes.
func (ps *Peers) PeersWithoutBlock(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.HasBlock(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTx retrieves a list of peers that do not have a given transaction
// in their set of known hashes.
func (ps *Peers) PeersWithoutTx(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.HasTransaction(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithoutTxPackage retrieves a list of peers that do not have a given tx package in
// their set of known hashes.
func (ps *Peers) PeersWithoutTxPackage(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if !p.HasTxPackage(hash) {
			list = append(list, p)
		}
	}
	return list
}

// PeersWithTxPackage retrieves a list of peers that have a given tx package in
// their set of known hashes.
func (ps *Peers) PeersWithTxPackage(hash common.Hash) []*Peer {
	ps.lock.RLock()
	defer ps.lock.RUnlock()

	list := make([]*Peer, 0, len(ps.peers))
	for _, p := range ps.peers {
		if p.HasTxPackage(hash) {
			list = append(list, p)
		}
	}
	return list
}

// Close disconnects all peers.
// No new peers can be registered after Close has returned.
func (ps *Peers) Close() {
	ps.lock.Lock()
	defer ps.lock.Unlock()

	for _, p := range ps.peers {
		p.Disconnect(p2p.DiscQuitting)
	}
	ps.closed = true
}

// newPeerSet creates a new peer set top track the active download sources.
func NewPeers() *Peers {
	return &Peers{
		peers: make(map[string]*Peer),
	}
}
