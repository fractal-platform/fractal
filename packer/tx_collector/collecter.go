// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package tx_collector contains the implementation of tx collector for packer.
package tx_collector

import (
	"bytes"
	"net"

	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/utils/log"
)

type TxCollector struct {
	listenAddr string
	listener   net.Listener
	txChan     chan types.Transactions
}

func NewTxCollector(listenAddr string) *TxCollector {
	return &TxCollector{
		listenAddr: listenAddr,
	}
}

func (s *TxCollector) Start(txChan chan types.Transactions) error {
	// Launch the TCP listener.
	listener, err := net.Listen("tcp", s.listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.txChan = txChan
	go s.listenLoop()
	return nil
}

func (s *TxCollector) Stop() {
	if s.txChan != nil {
		close(s.txChan)
	}
	if s.listener != nil {
		s.listener.Close()
	}
}

type tempError interface {
	Temporary() bool
}

func (s *TxCollector) listenLoop() {
	for {
		var (
			fd  net.Conn
			err error
		)
		for {
			fd, err = s.listener.Accept()
			if tempErr, ok := err.(tempError); ok && tempErr.Temporary() {
				log.Debug("Temporary read error", "err", err)
				continue
			} else if err != nil {
				log.Debug("Read error", "err", err)
				return
			}
			break
		}

		log.Debug("Accepted connection", "addr", fd.RemoteAddr())
		go s.handleConn(fd)
	}
}

func (s *TxCollector) handleConn(fd net.Conn) {
	for {
		msg, err := ReadMsg(fd)
		if err != nil {
			log.Error("TxCollector ReadMsg error", "addr", fd.RemoteAddr(), "err", err)
			break
		}

		if msg.Code != TxMsg || msg.Size == 0 {
			log.Error("TxCollector ReadMsg wrong data", "addr", fd.RemoteAddr(), "code", msg.Code, "size", msg.Size)
			continue
		}

		var txList types.Transactions
		if err := rlp.Decode(msg.Payload, &txList); err != nil {
			log.Error("TxCollector Decode Payload error", "addr", fd.RemoteAddr(), "err", err)
		}
		s.txChan <- txList

		// write back to client
		err = WriteMsg(fd, &Msg{
			Code:    TxConfirmMsg,
			Size:    0,
			Payload: bytes.NewReader([]byte{}),
		})
		if err != nil {
			log.Error("TxCollector WriteMsg error", "addr", fd.RemoteAddr(), "err", err)
			break
		}
	}
}
