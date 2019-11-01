// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package protocol contains the definitions of network protocol for fractal.
package protocol

import (
	"strconv"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
)

// Constants to match up protocol versions and messages
const (
	//ftl1 = 1
	ftl2 = 2
)

// ProtocolName is the official short name of the protocol used during capability negotiation.
var ProtocolName = "ftl"

// ProtocolVersions are the upported versions of the ftl protocol (first is primary).
var ProtocolVersions = []uint{ftl2}

const ProtocolMaxMsgSize = 512 * 1024 * 1024 // Maximum cap on the size of a protocol message

type MsgCode byte

// ftl protocol message codes
const (
	MsgCodeBegin = iota

	// for handshake
	StatusMsg

	// for block propagation
	NewBlockMsg

	//
	BlockReqMsg
	BlockRspMsg

	// for tx propagation
	TxMsg

	// for tx package propagation
	TxPackageHashMsg
	TxPackageReqMsg
	TxPackageRspMsg

	// for state sync
	NodeDataReqMsg
	NodeDataRspMsg

	// for hash list (cp2fp, fastsync, peersync)
	SyncHashListReqMsg
	SyncHashListRspMsg

	// for hash tree sync
	SyncHashTreeReqMsg
	SyncHashTreeRspMsg

	// for post fixpoint blocks sync
	SyncBestPeerBlocksReqMsg
	SyncBestPeerBlocksRspMsg

	// for block sync (cp2fp, fastsync, peersync)
	PkgsForBlockSyncReqMsg
	PkgsForBlockSyncRspMsg
	BlocksForBlockSyncReqMsg
	BlocksForBlockSyncRspMsg

	MsgCodeEnd
)

// ProtocolLengths are the number of implemented message corresponding to different protocol versions.
var ProtocolLengths = []uint64{MsgCodeEnd}

func (s MsgCode) String() string {
	if s <= MsgCodeBegin || s >= MsgCodeEnd {
		return "Unknown"
	}

	list := [...]string{
		"StatusMsg",
		"NewBlockMsg",
		"BlockReqMsg",
		"BlockRspMsg",
		"TxMsg",
		"TxPackageHashMsg",
		"TxPackageReqMsg",
		"TxPackageRspMsg",
		"NodeDataReqMsg",
		"NodeDataRspMsg",
		"SyncHashListReqMsg",
		"SyncHashListRspMsg",
		"SyncHashTreeReqMsg",
		"SyncHashTreeRspMsg",
		"SyncBestPeerBlocksReqMsg",
		"SyncBestPeerBlocksRspMsg",
		"PkgsForBlockSyncReqMsg",
		"PkgsForBlockSyncRspMsg",
		"BlocksForBlockSyncReqMsg",
		"BlocksForBlockSyncRspMsg"}
	return list[s-1]
}

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

type RequestData struct {
	ReqID uint64
}

//
type FetchBlockRsp struct {
	Blocks     []*types.Block
	TxPackages []*types.TxPackage
	RoundTo    uint64
	Finished   bool
}

type HashElem struct {
	Height  uint64
	Hash    common.Hash
	Round   uint64
	AccHash common.Hash
}

func (h HashElem) String() string {
	return strconv.FormatUint(h.Height, 10) + h.Hash.String() + strconv.FormatUint(h.Round, 10) + h.AccHash.String()
}

type HashElems []*HashElem

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

type SyncHashListReq struct {
	RequestData
	Stage      SyncStage
	Type       SyncHashType
	HashFrom   common.Hash
	HeightFrom uint64
	HashTo     common.Hash
	HeightTo   uint64
}

type SyncHashListRsp struct {
	RequestData
	Stage  SyncStage
	Type   SyncHashType
	Hashes HashElems
}

type SyncHashTreeReq struct {
	RequestData
	HashFrom common.Hash
	HashTo   common.Hash
}

type SyncHashTreeRsp struct {
	RequestData
	TreePoint types.TreePoint
	HashTree  types.HashTree
}

type IntervalHashReq struct {
	RequestData
	HashEFrom HashElem
	HashETo   HashElem
}

type SyncPkgsReq struct {
	RequestData
	Stage     SyncStage
	PkgHashes []common.Hash
}

type SyncPkgsRsp struct {
	RequestData
	Stage SyncStage
	Pkgs  types.TxPackages
}

type SyncBlocksReq struct {
	RequestData
	Stage     SyncStage
	HashReqs  []common.Hash
	RoundFrom uint64 // block from which to retrieve Blocks
	RoundTo   uint64 // block to which to retrieve Blocks
}

type SyncBlocksRsp struct {
	RequestData
	Stage     SyncStage
	Blocks    types.Blocks
	RoundFrom uint64 // block from which to retrieve Blocks
	RoundTo   uint64 // block to which to retrieve Blocks
}
