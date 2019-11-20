// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Package types contains data types related to Fractal consensus.
package types

import (
	"encoding/json"
	"io"
	"math/big"
	"sort"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rlp"
)

// BlockHeader represents the header of a block in the Fractal blockchain
type BlockHeader struct {
	ParentHash     common.Hash    `json:"parentHash"       gencodec:"required"`
	Round          uint64         `json:"round"            gencodec:"required"`
	Sig            []byte         `json:"sig"              gencodec:"required"`
	Coinbase       common.Address `json:"miner"            gencodec:"required"`
	Difficulty     *big.Int       `json:"difficulty"       gencodec:"required"`
	Height         uint64         `json:"height"           gencodec:"required"`
	Amount         uint64         `json:"amount"           gencodec:"required"`
	GasLimit       uint64         `json:"gasLimit"         gencodec:"required"`
	GasUsed        uint64         `json:"gasUsed"          gencodec:"required"`
	StateHash      common.Hash    `json:"stateHash"        gencodec:"required"`
	TxHash         common.Hash    `json:"txHash"           gencodec:"required"`
	ReceiptHash    common.Hash    `json:"receiptsRoot"     gencodec:"required"`
	ParentFullHash common.Hash    `json:"parentFullHash"   gencodec:"required"`
	Confirms       []common.Hash  `json:"confirms"`
	FullSig        []byte         `json:"fullSig"          gencodec:"required"`
	MinedTime      uint64         `json:"minedTime"        gencodec:"required"`
	HopCount       uint64         `json:"hopCount"         gencodec:"required"`
}

func (bh *BlockHeader) SimpleHash() common.Hash {
	v := common.RlpHash([]interface{}{
		bh.ParentHash,
		bh.Round,
		bh.Sig,
	})
	return v
}

// Hash returns the keccak256 hash of full block.
// The hash is computed on the first call and cached thereafter.
func (bh *BlockHeader) FullHash() common.Hash {
	v := common.RlpHash([]interface{}{
		bh.ParentHash,
		bh.Round,
		bh.Sig,
		bh.Coinbase,
		bh.Difficulty,
		bh.Height,
		bh.Amount,
		bh.GasLimit,
		bh.GasUsed,
		bh.StateHash,
		bh.TxHash,
		bh.ReceiptHash,
		bh.ParentFullHash,
		bh.Confirms,
	})
	return v
}

// BlockHeader represents the body of a block in the Fractal blockchain
type BlockBody struct {
	Transactions    []*Transaction `json:"transactions"`
	TxPackageHashes []common.Hash  `json:"txpackages"`
}

type BlockReceivePathEnum byte

const (
	BlockNull BlockReceivePathEnum = iota
	BlockFastSync
	BlockMined
)

type BlockStateCheckedEnum byte

const (
	NoBlockState BlockStateCheckedEnum = iota
	HasBlockStateButNotChecked
	BlockStateChecked
)

// Block represents a block in the Fractal blockchain.
type Block struct {
	// contents
	Header BlockHeader
	Body   BlockBody

	// caches
	simpleHash atomic.Value
	fullHash   atomic.Value
	bloom      atomic.Value

	// accHash
	AccHash common.Hash

	// whether the block state has been checked
	StateChecked BlockStateCheckedEnum

	//
	ReceivedAt   time.Time
	ReceivedFrom interface{}
	ReceivedPath BlockReceivePathEnum
}

func NewBlock(parentHash common.Hash, round uint64,
	sig []byte, coinbase common.Address, difficulty *big.Int, height uint64) *Block {
	block := &Block{
		Header: BlockHeader{
			ParentHash: parentHash,
			Round:      round,
			Sig:        sig,
			Coinbase:   coinbase,
			Difficulty: difficulty,
			Height:     height,
		},
	}
	return block
}

func NewBlockWithHeader(header *BlockHeader) *Block {
	block := &Block{
		Header: *header,
	}
	return block
}

func (b *Block) SignHashByte() []byte {
	v := common.RlpHash([]interface{}{
		b.Header.ParentHash,
		b.Header.Round,
	})
	return v[:]
}

// Hash returns the keccak256 hash of block's consensus part.
// The hash is computed on the first call and cached thereafter.
func (b *Block) SimpleHash() common.Hash {
	if hash := b.simpleHash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.Header.SimpleHash()
	b.simpleHash.Store(v)
	return v
}

// Hash returns the keccak256 hash of full block.
// The hash is computed on the first call and cached thereafter.
func (b *Block) FullHash() common.Hash {
	if hash := b.fullHash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := b.Header.FullHash()
	b.fullHash.Store(v)
	return v
}

func (b *Block) CacheBloom(bloom *Bloom) {
	b.bloom.Store(bloom)
}

func (b *Block) Bloom() *Bloom {
	if bloom := b.bloom.Load(); bloom != nil {
		return bloom.(*Bloom)
	}
	return nil
}

// for block serialization
type blockContent struct {
	Header BlockHeader
	Body   BlockBody
}

// DecodeRLP decodes the fractal block
func (b *Block) DecodeRLP(s *rlp.Stream) error {
	var content blockContent
	if err := s.Decode(&content); err != nil {
		return err
	}
	b.Header = content.Header
	b.Body = content.Body
	return nil
}

// EncodeRLP serializes block into the RLP format.
func (b *Block) EncodeRLP(w io.Writer) error {
	var content = blockContent{
		Header: b.Header,
		Body:   b.Body,
	}
	return rlp.Encode(w, content)
}

// Serialize block to json format
func (b *Block) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"Header":     b.Header,
		"Body":       b.Body,
		"FullHash":   b.FullHash(),
		"SimpleHash": b.SimpleHash(),
	})
}

// return 1 if b2 is behind b
func (b *Block) CompareByRoundAndSimpleHash(b2 *Block) int {
	if b.Header.Round > b2.Header.Round {
		return 1
	} else if b.Header.Round < b2.Header.Round {
		return -1
	} else {
		if b.Header.SimpleHash().Hex() > b2.Header.SimpleHash().Hex() {
			return 1
		} else if b.Header.SimpleHash().Hex() < b2.Header.SimpleHash().Hex() {
			return -1
		} else {
			return 0
		}
	}
}

// return 1 if b is better then b2
func (b *Block) CompareByHeightAndRoundAndSimpleHash(b2 *Block) int {
	if b.Header.Height > b2.Header.Height {
		return 1
	} else if b.Header.Height < b2.Header.Height {
		return -1
	} else {
		return b.CompareByRoundAndSimpleHash(b2)
	}
}

// CalcGasLimit computes the gas limit of the next block after parent.
// This is miner strategy, not consensus protocol.
func CalcGasLimit(parent *Block) uint64 {
	// contrib = (parentGasUsed * 3 / 2) / 1024
	contrib := (parent.Header.GasUsed + parent.Header.GasUsed/2) / params.GasLimitBoundDivisor

	// decay = parentGasLimit / 1024 -1
	decay := parent.Header.GasLimit/params.GasLimitBoundDivisor - 1

	/*
		strategy: gasLimit of block-to-mine is set based on parent's
		gasUsed value.  if parentGasUsed > parentGasLimit * (2/3) then we
		increase it, otherwise lower it (or leave it unchanged if it's right
		at that usage) the amount increased/decreased depends on how far away
		from parentGasLimit * (2/3) parentGasUsed is.
	*/
	limit := parent.Header.GasLimit - decay + contrib
	if limit < params.MinGasLimit {
		limit = params.MinGasLimit
	}
	//// however, if we're now below the target (TargetGasLimit) we increase the
	//// limit as much as we can (parentGasLimit / 1024 -1)
	//if limit < params.TargetGasLimit {
	//	limit = parent.Header.GasLimit + decay
	//	if limit > params.TargetGasLimit {
	//		limit = params.TargetGasLimit
	//	}
	//}
	return limit
}

// type for block array
type Blocks []*Block

// sort blocks by round
func (blocks *Blocks) SortByRoundHash() {
	sort.Slice(*blocks, func(i, j int) bool {
		if (*blocks)[i].Header.Round == (*blocks)[j].Header.Round {
			return (*blocks)[i].SimpleHash().Hex() < (*blocks)[j].SimpleHash().Hex()
		}
		return (*blocks)[i].Header.Round < (*blocks)[j].Header.Round
	})
}

//
func (blocks *Blocks) Has(hash common.Hash) bool {
	if blocks == nil {
		return false
	}
	for _, block := range *blocks {
		if block.FullHash() == hash {
			return true
		}
	}
	return false
}

// remove block from array
func (blocks *Blocks) Remove(hash common.Hash) {
	for i, block := range *blocks {
		if block.FullHash() == hash {
			*blocks = append((*blocks)[:i], (*blocks)[i+1:]...)
			return
		}
	}
}

//
func (blocks *Blocks) Copy() Blocks {
	ret := make(Blocks, 0)
	ret = append(ret, *blocks...)
	return ret
}

// type for block complex index
type BlockRoundHash struct {
	Round      uint64
	SimpleHash common.Hash
	FullHash   common.Hash
}

// type for block index list
type BlockRoundHashes []*BlockRoundHash

// sort blocks by round
func (list *BlockRoundHashes) SortByRoundHash() {
	sort.Slice(*list, func(i, j int) bool {
		if (*list)[i].Round == (*list)[j].Round {
			return (*list)[i].SimpleHash.Hex() < (*list)[j].SimpleHash.Hex()
		}
		return (*list)[i].Round < (*list)[j].Round
	})
}
