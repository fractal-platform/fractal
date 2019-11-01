package wasm

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

// for transaction call
type callframe struct {
	depth           uint8          // call depth(minimize: 0)
	storageDelegate bool           // do not change storage context address if true
	userDelegate    bool           // do not change business user address if true
	from            common.Address // from address(caller)
	to              common.Address // contract address(callee)
	storage         common.Address // storage address
	user            common.Address // business user address(maybe any caller in the call stack)
	action          []byte         // call action
	ret             int            // action return (0=success)
	err             string         // error string if call failed
	result          []byte         // action call result
	gas             uint64         // gas used
}

type registerParam struct {
	stateDb     *state.StateDB
	block       *types.Block
	remainedGas *uint64
	callstack   []callframe
	lastframe   callframe
}

type RegisterParam struct {
	lock sync.RWMutex
	item map[uint64]*registerParam

	nextKey uint64
}

var once sync.Once

var GlobalRegisterParam *RegisterParam

func GetGlobalRegisterParam() *RegisterParam {
	once.Do(func() {
		GlobalRegisterParam = &RegisterParam{
			item: make(map[uint64]*registerParam),
		}
	})
	return GlobalRegisterParam
}

func (r *RegisterParam) RegisterParam(s *state.StateDB, b *types.Block) uint64 {
	r.lock.Lock()
	defer r.lock.Unlock()

	key := atomic.AddUint64(&r.nextKey, 1)
	r.item[key] = &registerParam{
		stateDb:   s,
		block:     b,
		callstack: make([]callframe, 0),
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

func (r *RegisterParam) GetContractCode(key uint64, address common.Address) []byte {
	return r.getState(key).GetCode(address)
}

func (r *RegisterParam) GetContractOwner(key uint64, address common.Address) common.Address {
	return r.getState(key).GetContractOwner(address)
}

func (r *RegisterParam) GetCurrentContract(key uint64) common.Address {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.item[key].callstack)
	return r.item[key].callstack[size-1].to
}

func (r *RegisterParam) GetCurrentUser(key uint64) common.Address {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.item[key].callstack)
	return r.item[key].callstack[size-1].user
}

func (r *RegisterParam) GetCurrentStorage(callbackParamKey uint64) common.Address {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.item[callbackParamKey].callstack)
	return r.item[callbackParamKey].callstack[size-1].storage
}

func (r *RegisterParam) GetCurrentDepth(key uint64) uint8 {
	r.lock.RLock()
	defer r.lock.RUnlock()

	size := len(r.item[key].callstack)
	return r.item[key].callstack[size-1].depth
}

func (r *RegisterParam) GetRemainedGas(key uint64) *uint64 {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.item[key].remainedGas
}

func (r *RegisterParam) SetRemainedGas(key uint64, remainedGas *uint64) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	r.item[key].remainedGas = remainedGas
}

func (r *RegisterParam) ClearCallstack(key uint64) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	r.item[key].callstack = make([]callframe, 0)
}

func (r *RegisterParam) AddCallstack(key uint64, from common.Address, to common.Address, user common.Address, action []byte, storageDelegate bool, userDelegate bool) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	storageAddress := to
	if storageDelegate {
		storageAddress = from
	}

	userAddress := user
	if userDelegate {
		userAddress = from
	}

	depth := uint8(len(r.item[key].callstack))
	r.item[key].lastframe = callframe{}
	r.item[key].callstack = append(r.item[key].callstack, callframe{
		depth:           depth,
		storageDelegate: storageDelegate,
		userDelegate:    userDelegate,
		from:            from,
		to:              to,
		storage:         storageAddress,
		user:            userAddress,
		action:          action,
		ret:             0,
		err:             "",
		result:          nil,
		gas:             0,
	})
}

func (r *RegisterParam) FulfillCallstack(key uint64, ret int, err string, gas uint64) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	index := len(r.item[key].callstack) - 1
	r.item[key].callstack[index].ret = ret
	r.item[key].callstack[index].err = err
	r.item[key].callstack[index].gas = gas

	r.item[key].lastframe = r.item[key].callstack[index]
	r.item[key].callstack = r.item[key].callstack[:index]
}

func (r *RegisterParam) GetCallResult(key uint64) []byte {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.item[key].lastframe.result
}

func (r *RegisterParam) SetCallResult(key uint64, result []byte) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	index := len(r.item[key].callstack) - 1
	r.item[key].callstack[index].result = result
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

func GetBlockHash(callbackParamKey uint64) common.Hash {
	b := GetGlobalRegisterParam().getBlock(callbackParamKey)
	if b == nil {
		log.Error("GetBlockHeight error: block is nil")
		return common.Hash{}
	}

	return b.SimpleHash()
}

func AddLog(callbackParamKey uint64, address common.Address, topicSlice []byte, topicNum int, dataSlice []byte, dataLength int) {
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

	var data = make([]byte, dataLength)
	copy(data, dataSlice)

	s.AddLog(&types.Log{
		Address:     address,
		Topics:      topics,
		Data:        data,
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
