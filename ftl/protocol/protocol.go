// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package network contains the implementation of network protocol handler for fractal.
package protocol

import (
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"strconv"
)

// Constants to match up protocol versions and messages
const (
	ftl2 = 2
)

// ProtocolName is the official short name of the protocol used during capability negotiation.
var ProtocolName = "ftl"

// ProtocolVersions are the upported versions of the ftl protocol (first is primary).
var ProtocolVersions = []uint{ftl2}

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{20}

const ProtocolMaxMsgSize = 512 * 1024 * 1024 // Maximum cap on the size of a protocol message

// ftl protocol message codes
const (
	// Protocol messages belonging to ftl1
	StatusMsg = 0x00

	// for block propagation
	NewBlockMsg = 0x01

	//
	GetBlocksMsg = 0x02
	BlocksMsg    = 0x03

	// for tx propagation
	TxMsg = 0x04

	// for tx package propagation
	TxPackageHashMsg = 0x05
	GetTxPackageMsg  = 0x06
	TxPackageMsg     = 0x07

	// for state sync
	GetNodeDataMsg = 0x08
	NodeDataMsg    = 0x09

	// for hash list (cp2fp, fastsync, peersync)
	SyncHashListReqMsg = 0x0a
	SyncHashListResMsg = 0x0b

	// for pre blocks sync
	SyncPreBlocksForStateReqMsg = 0x0c
	SyncPreBlocksForStateRspMsg = 0x0d

	// for post blocks sync
	SyncPostBlocksForStateReqMsg = 0x0e
	SyncPostBlocksForStateRspMsg = 0x0f

	// for block sync (cp2fp, fastsync, peersync)
	GetPkgsForBlockSyncMsg   = 0x10
	PkgsForBlockSyncMsg      = 0x11
	GetBlocksForBlockSyncMsg = 0x12
	BlocksForBlockSyncMsg    = 0x13
)

type SyncStage byte

const (
	SyncStageBegin SyncStage = iota
	SyncStageCP2FP
	SyncStageFastSync
	SyncStagePeerSync
	SyncStageEnd
)

func (s SyncStage) String() string {
	if s <= SyncStageBegin || s >= SyncStageEnd {
		return "Unknown"
	}

	list := [...]string{
		"CP2FP",
		"FastSync",
		"PeerSync"}
	return list[s-1]
}

type SyncHashType byte

const (
	SyncHashTypeBegin SyncHashType = iota
	SyncHashTypeLong
	SyncHashTypeShort
	SyncHashTypeEnd
)

func (s SyncHashType) String() string {
	if s <= SyncHashTypeBegin || s >= SyncHashTypeEnd {
		return "Unknown"
	}

	list := [...]string{
		"Long",
		"Short"}
	return list[s-1]
}

type ErrCode int

const (
	ErrMsgTooLarge = iota
	ErrDecode
	ErrInvalidMsgCode
	ErrProtocolVersionMismatch
	ErrNetworkIdMismatch
	ErrGenesisBlockMismatch
	ErrNoStatusMsg
	ErrExtraStatusMsg
	ErrSuspendedPeer
	ErrInvalidRepSyncBlockMsg
)

func (e ErrCode) String() string {
	return errorToString[int(e)]
}

// XXX change once legacy code is out
var errorToString = map[int]string{
	ErrMsgTooLarge:             "Message too long",
	ErrDecode:                  "Invalid message",
	ErrInvalidMsgCode:          "Invalid message code",
	ErrProtocolVersionMismatch: "Protocol version mismatch",
	ErrNetworkIdMismatch:       "NetworkId mismatch",
	ErrGenesisBlockMismatch:    "Genesis block mismatch",
	ErrNoStatusMsg:             "No status message",
	ErrExtraStatusMsg:          "Extra status message",
	ErrSuspendedPeer:           "Suspended peer",
	ErrInvalidRepSyncBlockMsg:  "Invaild type of Msg for requesting Sync block",
}

// statusData is the network packet for the status message.
type StatusData struct {
	ProtocolVersion   uint32
	NetworkId         uint64
	Round             uint64
	Height            uint64
	CurrentFullHash   common.Hash
	CurrentSimpleHash common.Hash
	GenesisHash       common.Hash
}

// newBlockData is the network packet for the block propagation message.
type NewBlockData struct {
	Block  *types.Block
	Height uint64
}

// getBlocksData represents a block query.
type GetBlocksData struct {
	OriginHash common.Hash // block from which to retrieve Blocks
	Depth      uint64
	Reverse    bool
	RoundFrom  uint64 // block from which to retrieve Blocks
	RoundTo    uint64 // block to which to retrieve Blocks
}

//
type FetchBlockRsp struct {
	Blocks     []*types.Block
	TxPackages []*types.TxPackage
	RoundTo    uint64
	Finished   bool
}

type HashElem struct {
	Height uint64
	Hash   common.Hash
	Round  uint64
}

func (h HashElem) String() string {
	return strconv.FormatUint(h.Height, 10) + h.Hash.String() + strconv.FormatUint(h.Round, 10)
}

type HashElems []*HashElem

//func (h HashElems) reverse() HashElems {
//	var resultHList HashElems
//	if len(h) == 0 {
//		return h
//	} else {
//		for i := len(h) - 1; i >= 0; i-- {
//			resultHList = append(resultHList, h[i])
//		}
//	}
//	return resultHList
//}

type SyncHashListReq struct {
	Stage      SyncStage
	Type       SyncHashType
	HashFrom   common.Hash
	HeightFrom uint64
	HashTo     common.Hash
	HeightTo   uint64
}

type SyncHashListRsp struct {
	Stage  SyncStage
	Type   SyncHashType
	Hashes HashElems
}

func (h HashElem) CompareTo(h1 HashElem) int {
	if h.Height > h1.Height {
		return 1
	} else if h.Height == h1.Height {
		if h.Round < h1.Round {
			return 1
		} else if h.Round == h1.Round {
			if h.Hash.Hex() < h1.Hash.Hex() {
				return 1
			} else if h.Hash.Hex() == h1.Hash.Hex() {
				return 0
			}
		}
	}
	return -1
}

type IntervalHashReq struct {
	HashEFrom HashElem
	HashETo   HashElem
}

type SyncPkgsReq struct {
	Stage     SyncStage
	PkgHashes []common.Hash
}

type SyncPkgsRsp struct {
	Stage SyncStage
	Pkgs  types.TxPackages
}

type SyncBlocksReq struct {
	Stage     SyncStage
	RoundFrom uint64 // block from which to retrieve Blocks
	RoundTo   uint64 // block to which to retrieve Blocks
}

type SyncBlocksRsp struct {
	Stage     SyncStage
	Blocks    types.Blocks
	RoundFrom uint64 // block from which to retrieve Blocks
	RoundTo   uint64 // block to which to retrieve Blocks
}
