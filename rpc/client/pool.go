// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package rpcclient contains implementations for net rpc client.
package rpcclient

import (
	"github.com/fractal-platform/fractal/utils/log"
	"sync"
	"time"
)

type RpcConnection struct {
	Client   *Client
	idle     bool
	timer    *time.Timer
	lifetime int
}

func (c *RpcConnection) Release() {
	c.timer.Reset(time.Second * time.Duration(c.lifetime))
	c.idle = true
}

type RpcConnPool struct {
	lifetime int
	connMap  map[string][]*RpcConnection
	mutex    sync.Mutex
}

func NewRpcConnPool(lifetime int) *RpcConnPool {
	pool := &RpcConnPool{
		lifetime: lifetime,
		connMap:  make(map[string][]*RpcConnection),
	}
	go pool.loop()
	return pool
}

func (p *RpcConnPool) loop() {
	for {
		p.mutex.Lock()
		for _, conns := range p.connMap {
			for i, conn := range conns {
				if conn.idle {
					select {
					case <-conn.timer.C:
						conns = append(conns[:i], conns[i+1:]...)
						break
					default:
						break
					}
				}
			}
		}
		p.mutex.Unlock()

		t := time.NewTimer(time.Second * 60)
		<-t.C
	}
}

func (p *RpcConnPool) FetchRpcConn(rpcAddr string) *RpcConnection {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	conns, ok := p.connMap[rpcAddr]
	if !ok {
		conns = make([]*RpcConnection, 0)
	}

	for _, conn := range conns {
		if conn.idle {
			conn.idle = false
			return conn
		}
	}

	client, err := Dial(rpcAddr)
	if err != nil {
		log.Error("connect to rpc error", "rpc", rpcAddr, "err", err)
	}
	conn := &RpcConnection{
		Client:   client,
		idle:     false,
		timer:    time.NewTimer(time.Second * time.Duration(p.lifetime)),
		lifetime: p.lifetime,
	}
	conns = append(conns, conn)
	p.connMap[rpcAddr] = conns
	return conn
}
