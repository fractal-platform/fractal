// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"bytes"
	"context"
	"errors"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/rpc/server"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
)

// BlockChainAPI provides an API to access the Fractal blockchain.
// It offers only methods that operate on public data that is freely available to anyone.
type BlockChainAPI struct {
	ftl fractal
}

// NewBlockChainAPI creates a new Fractal blockchain API.
func NewBlockChainAPI(ftl fractal) *BlockChainAPI {
	return &BlockChainAPI{ftl}
}

// HeadBlock returns the head block in the chain.
func (s *BlockChainAPI) HeadBlock() *types.Block {
	return s.ftl.BlockChain().CurrentBlock()
}

// Genesis returns the first block in the chain.
func (s *BlockChainAPI) Genesis() *types.Block {
	return s.ftl.BlockChain().Genesis()
}

// GetBlock returns the block with the hash.
func (s *BlockChainAPI) GetBlock(hash common.Hash) *types.Block {
	return s.ftl.BlockChain().GetBlock(hash)
}

func (s *BlockChainAPI) GetBlockByHeight(height hexutil.Uint64) *types.Block {
	header, err := s.ftl.BlockChain().GetMainBranchBlock(uint64(height))
	if err != nil {
		log.Info("BlockChainAPI GetBlockByHeight error", "err", err)
		return nil
	}
	block := types.NewBlockWithHeader(header)
	block.Body = *dbaccessor.ReadBlockBody(s.ftl.ChainDb(), header.FullHash())
	return block
}

// BlockHeight returns the block heigth of the chain head.
func (s *BlockChainAPI) BlockHeight() hexutil.Uint64 {
	return hexutil.Uint64(s.ftl.BlockChain().CurrentBlock().Header.Height)
}

func (s *BlockChainAPI) GetBackwardBlocks(fullHash common.Hash, num uint32) types.Blocks {
	block := s.ftl.BlockChain().GetBlock(fullHash)
	return s.ftl.BlockChain().GetBackwardBlocks(block, uint64(num))
}

func (s *BlockChainAPI) GetAncestorBlocks(fullHash common.Hash, num uint32) types.Blocks {
	return s.ftl.BlockChain().GetAncestorBlocksFromBlock(fullHash, uint64(num))
}

func (s *BlockChainAPI) GetDescendantBlocks(fullHash common.Hash, num uint32) types.Blocks {
	return s.ftl.BlockChain().GetDescendantBlocksFromBlock(fullHash, uint64(num))
}

func (s *BlockChainAPI) GetNearbyBlocks(fullHash common.Hash, num uint32) types.Blocks {
	return s.ftl.BlockChain().GetNearbyBlocksFromBlock(fullHash, uint64(num))
}

// SubNewBlock provides information when new block arrived
func (s *BlockChainAPI) SubNewBlock(ctx context.Context) (*rpcserver.Subscription, error) {
	notifier, supported := rpcserver.NotifierFromContext(ctx)
	if !supported {
		return &rpcserver.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()

	go func() {
		ch := make(chan types.ChainUpdateEvent)
		sub := s.ftl.BlockChain().SubscribeChainUpdateEvent(ch)

		for {
			select {
			case e := <-ch:
				notifier.Notify(rpcSub.ID, e.Block)
			case <-rpcSub.Err():
				sub.Unsubscribe()
				return
			case <-notifier.Closed():
				sub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}

// GetBalance returns the amount of nFra for the given address in the state of the given block
func (s *BlockChainAPI) GetBalance(ctx context.Context, address common.Address, blockHashStr string) (*hexutil.Big, error) {
	block := s.ftl.GetBlockStr(blockHashStr)
	if block == nil {
		return nil, errors.New("block not found")
	}
	state, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if state == nil || err != nil {
		return nil, err
	}
	return (*hexutil.Big)(state.GetBalance(address)), state.Error()
}

func (s *BlockChainAPI) GetStorageAt(ctx context.Context, address common.Address, table string, key hexutil.Bytes, blockHashStr string) (hexutil.Bytes, error) {
	block := s.ftl.GetBlockStr(blockHashStr)
	if block == nil {
		return nil, errors.New("block not found")
	}
	stateDb, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if stateDb == nil || err != nil {
		return nil, err
	}

	t, _ := utils.String2Uint64(table)
	storageKey := state.GetStorageKey(t, key)
	value := stateDb.GetState(address, storageKey)
	log.Info("GetStorageAt", "address", hexutil.Encode(address[:]), "storageKey", hexutil.Encode(storageKey.ToSlice()), "value", value)
	return value, nil
}

func (s *BlockChainAPI) GetCode(ctx context.Context, address common.Address, blockHashStr string) (hexutil.Bytes, error) {
	block := s.ftl.GetBlockStr(blockHashStr)
	if block == nil {
		return nil, errors.New("block not found")
	}
	stateDb, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if stateDb == nil || err != nil {
		return nil, err
	}

	return stateDb.GetCode(address), nil
}

func (s *BlockChainAPI) GetContractOwner(ctx context.Context, contractAddress common.Address) (common.Address, error) {
	block := s.ftl.BlockChain().CurrentBlock()
	if block == nil {
		return common.Address{}, errors.New("block not found")
	}
	stateDb, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if stateDb == nil || err != nil {
		return common.Address{}, err
	}

	return stateDb.GetContractOwner(contractAddress), nil
}

func (s *BlockChainAPI) GetTransferWhiteList(ctx context.Context, blockHashStr string) (string, error) {
	block := s.ftl.GetBlockStr(blockHashStr)
	if block == nil {
		return "", errors.New("block not found")
	}
	stateDb, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if stateDb == nil || err != nil {
		return "", err
	}

	whiteList := stateDb.GetTransferWhiteList()

	if len(whiteList) == 0 {
		return "", nil
	}
	var buffer bytes.Buffer
	var i int
	for ; i < len(whiteList)-1; i++ {
		buffer.WriteString(whiteList[i].String())
		buffer.WriteString("\n")
	}
	buffer.WriteString(whiteList[i].String())

	return buffer.String(), nil
}

func (s *BlockChainAPI) GetTransferBlackList(ctx context.Context, blockHashStr string) (string, error) {
	block := s.ftl.GetBlockStr(blockHashStr)
	if block == nil {
		return "", errors.New("block not found")
	}
	stateDb, err := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	if stateDb == nil || err != nil {
		return "", err
	}

	blackList := stateDb.GetTransferBlackList()

	if len(blackList) == 0 {
		return "", nil
	}
	var buffer bytes.Buffer
	var i int
	for ; i < len(blackList)-1; i++ {
		buffer.WriteString(blackList[i].String())
		buffer.WriteString("\n")
	}
	buffer.WriteString(blackList[i].String())

	return buffer.String(), nil
}
