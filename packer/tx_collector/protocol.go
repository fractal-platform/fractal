// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package tx_collector contains the implementation of tx collector for packer.
package tx_collector

import (
	"bytes"
	"encoding/binary"
	"io"
	"net"
)

type Msg struct {
	Code    uint64
	Size    uint64
	Payload io.Reader
}

// tx collect message codes
const (
	TxMsg        = 0x00
	TxConfirmMsg = 0x01
)

func ReadMsg(fd net.Conn) (msg Msg, err error) {
	codeBuf := make([]byte, 8)
	if _, err := io.ReadFull(fd, codeBuf); err != nil {
		return msg, err
	}
	msg.Code = binary.BigEndian.Uint64(codeBuf)

	sizeBuf := make([]byte, 8)
	if _, err := io.ReadFull(fd, sizeBuf); err != nil {
		return msg, err
	}
	msg.Size = binary.BigEndian.Uint64(sizeBuf)

	if msg.Size > 0 {
		payload := make([]byte, msg.Size)
		if _, err := io.ReadFull(fd, payload); err != nil {
			return msg, err
		}
		msg.Payload = bytes.NewReader(payload)
	}
	return msg, nil
}

func WriteMsg(fd net.Conn, msg *Msg) error {
	codeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(codeBuf, msg.Code)
	sizeBuf := make([]byte, 8)
	binary.BigEndian.PutUint64(sizeBuf, msg.Size)

	var buf bytes.Buffer
	buf.Write(codeBuf)
	buf.Write(sizeBuf)
	if msg.Size > 0 {
		buf.ReadFrom(msg.Payload)
	}

	if _, err := fd.Write(buf.Bytes()); err != nil {
		return err
	}
	return nil
}
