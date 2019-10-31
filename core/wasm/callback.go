package wasm

import "C"
import (
	"math/big"
	"sync"
	"sync/atomic"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

type registerParam struct {
	stateDb *state.StateDB
	block   *types.Block
}

type RegisterParam struct {
	lock sync.RWMutex
	item map[uint64]registerParam

	nextKey uint64
}

var once sync.Once

var GlobalRegisterParam *RegisterParam

func GetGlobalRegisterParam() *RegisterParam {
	once.Do(func() {
		GlobalRegisterParam = &RegisterParam{
			item: make(map[uint64]registerParam),
		}
	})
	return GlobalRegisterParam
}

func (r *RegisterParam) RegisterParam(s *state.StateDB, b *types.Block) uint64 {
	r.lock.Lock()
	defer r.lock.Unlock()

	key := atomic.AddUint64(&r.nextKey, 1)
	r.item[key] = registerParam{
		stateDb: s,
		block:   b,
	}
	return key
}

func (r *RegisterParam) UnRegisterParam(key uint64) {
	r.lock.Lock()
	defer r.lock.Unlock()

	delete(r.item, key)
}

func (r *RegisterParam) getState(key uint64) *state.StateDB {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.item[key].stateDb
}

func (r *RegisterParam) getBlock(key uint64) *types.Block {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.item[key].block
}

func DbStore(callbackParamKey uint64, address common.Address, table uint64, key []byte, value []byte) {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbStore error: state is nil")
		return
	}

	storageKey := state.GetStorageKey(table, key)
	log.Info("DbStore", "address", hexutil.Encode(address[:]), "storageKey", hexutil.Encode(storageKey.ToSlice()), "value", value)
	s.SetState(address, storageKey, value)
}

func DbLoad(callbackParamKey uint64, address common.Address, table uint64, key []byte) []byte {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbLoad error: state is nil")
		return []byte{}
	}

	storageKey := state.GetStorageKey(table, key)
	value := s.GetState(address, storageKey)
	log.Info("DbLoad", "address", hexutil.Encode(address[:]), "storageKey", hexutil.Encode(storageKey.ToSlice()), "value", value)
	return value
}

func DbHasKey(callbackParamKey uint64, address common.Address, table uint64, key []byte) int {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbHasKey error: state is nil")
		return 0
	}

	stateObject := s.GetOrNewStateObject(address)
	if stateObject != nil {
		storageKey := state.GetStorageKey(table, key)
		return stateObject.HasKey(s.Database(), storageKey)
	}
	return 0
}

func DbRemoveKey(callbackParamKey uint64, address common.Address, table uint64, key []byte) {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbRemoveKey error: state is nil")
		return
	}

	stateObject := s.GetOrNewStateObject(address)
	if stateObject != nil {
		storageKey := state.GetStorageKey(table, key)
		stateObject.RemoveKey(s.Database(), storageKey)
	}
}

func DbHasTable(callbackParamKey uint64, address common.Address, table uint64) int {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbHasTable error: state is nil")
		return 0
	}

	stateObject := s.GetOrNewStateObject(address)
	if stateObject != nil {
		return stateObject.HasTable(s.Database(), table)
	}
	return 0
}

func DbRemoveTable(callbackParamKey uint64, address common.Address, table uint64) {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("DbRemoveTable error: state is nil")
		return
	}

	stateObject := s.GetOrNewStateObject(address)
	if stateObject != nil {
		stateObject.RemoveTable(s.Database(), table)
	}
}

func GetBlockRound(callbackParamKey uint64) uint64 {
	b := GetGlobalRegisterParam().getBlock(callbackParamKey)
	if b == nil {
		log.Error("GetBlockRound error: block is nil")
		return 0
	}

	return b.Header.Round / params.RoundsPerSecond
}

func GetBlockHeight(callbackParamKey uint64) uint64 {
	b := GetGlobalRegisterParam().getBlock(callbackParamKey)
	if b == nil {
		log.Error("GetBlockHeight error: block is nil")
		return 0
	}

	return b.Header.Height
}

func AddLog(callbackParamKey uint64, address common.Address, topicSlice []byte, topicNum int) {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("AddLog error: state is nil")
		return
	}
	b := GetGlobalRegisterParam().getBlock(callbackParamKey)
	if b == nil {
		log.Error("AddLog error: block is nil")
		return
	}
	blockHeight := b.Header.Height

	var topics = make([]common.Hash, topicNum)
	for i := range topics {
		topicStart := i * common.HashLength
		copy(topics[i][:], topicSlice[topicStart:topicStart+common.HashLength])
	}

	s.AddLog(&types.Log{
		Address:     address,
		Topics:      topics,
		Data:        nil,
		BlockNumber: blockHeight,
	})
}

func Transfer(callbackParamKey uint64, from common.Address, to common.Address, amount uint64, remainedGas *uint64) int {
	s := GetGlobalRegisterParam().getState(callbackParamKey)
	if s == nil {
		log.Error("Transfer error: state is nil")
		return -1
	}
	value := new(big.Int).SetUint64(amount)

	// check gas
	if *remainedGas < params.TxGas {
		log.Error("Transfer error: out of gas", "remainedGas", *remainedGas)
		return -1
	}
	// check transfer
	if s.GetBalance(from).Cmp(value) < 0 {
		log.Error("Transfer error: insufficient balance")
		return -1
	}

	// do
	*remainedGas -= params.TxGas
	s.SubBalance(from, value)
	s.AddBalance(to, value)
	return 0
}
