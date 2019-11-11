package txexec

/*
#cgo CFLAGS: -I./
#cgo LDFLAGS: -L./ -lwasmlib
#include <stdio.h>
#include <stdlib.h>

void c_db_store(unsigned long long callbackParamKey, unsigned long long table, char *key, int keyLength, char *value, int valueLength);
int c_db_load(unsigned long long callbackParamKey, unsigned long long table, char *key, int keyLength, char *value, int valueLength);
int c_db_has_key(unsigned long long callbackParamKey, unsigned long long table, char *key, int keyLength);
void c_db_remove_key(unsigned long long callbackParamKey, unsigned long long table, char *key, int keyLength);
int c_db_has_table(unsigned long long callbackParamKey, unsigned long long table);
void c_db_remove_table(unsigned long long callbackParamKey, unsigned long long table);
unsigned long long c_chain_current_time(unsigned long long callbackParamKey);
unsigned long long c_chain_current_height(unsigned long long callbackParamKey);
void c_chain_current_hash(unsigned long long callbackParamKey, char *simpleHash);
void c_add_log(unsigned long long callbackParamKey, char *topic, int topicNum, char *data, int dataLength);
int c_transfer(unsigned long long callbackParamKey, char *to, unsigned long long amount);
int c_call_action(unsigned long long callbackParamKey, char *to, char *actionBytes, int actionLength, unsigned long long amount, int storageDelegate, int userDelegate);
int c_call_result(unsigned long long callbackParamKey, char *value, int valueLength);
int c_set_result(unsigned long long callbackParamKey, char *value, int valueLength);
unsigned char c_call_depth(unsigned long long callbackParamKey);
int c_sha256(char *input, int length, char *hash);

typedef struct {
	void *cb_store;
	void *cb_load;
	void *cb_has_key;
	void *cb_remove_key;
	void *cb_has_table;
	void *cb_remove_table;
	void *cb_current_time;
	void *cb_current_height;
	void *cb_current_hash;
	void *cb_add_log;
	void *c_transfer;
	void *c_call_action;
	void *c_call_result;
	void *c_set_result;
	void *c_sha256;
} Callbacks;

int execute(unsigned char *codeBytes, int codeLength, unsigned char *actionBytes, int actionLength, unsigned char *fromAddrBytes, unsigned char *toAddrBytes, unsigned char *owner, unsigned char *user, unsigned long long amount, unsigned long long *remainedGas, unsigned long long callbackParamKey, Callbacks *callbacks);

static inline int execute_go(unsigned char *codeBytes, int codeLength, unsigned char *actionBytes, int actionLength, unsigned char *fromAddrBytes, unsigned char *toAddrBytes, unsigned char *owner, unsigned char *user, unsigned long long amount, unsigned long long *remainedGas, unsigned long long callbackParamKey) {
	Callbacks callbacks= {	c_db_store, c_db_load, c_db_has_key, c_db_remove_key,
							c_db_has_table, c_db_remove_table, c_chain_current_time, c_chain_current_height, c_chain_current_hash,
							c_add_log, c_transfer, c_call_action, c_call_result, c_set_result, c_sha256 };
	return execute(codeBytes, codeLength, actionBytes, actionLength, fromAddrBytes, toAddrBytes, owner, user, amount, remainedGas, callbackParamKey, &callbacks);
}
*/
import "C"
import (
	"errors"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/crypto/sha3"
	"github.com/fractal-platform/fractal/utils/log"
	"unsafe"
)

const MaxWasmCallDepth = 8

type WasmExecutor struct {
	signer       types.Signer
	maxBitLength uint64
}

func NewWasmExecutor(signer types.Signer, maxBitLength uint64) TxExecutor {
	log.Info("NewExecutor: Init WasmExecutor")
	return &WasmExecutor{
		signer:       signer,
		maxBitLength: maxBitLength,
	}
}

func (e *WasmExecutor) ExecuteTxPackages(txpkgs types.TxPackages, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	var (
		newExecutedTxs  = executedTxs
		newAllLogs      = allLogs
		newReceipts     = receipts
		gasLimitReached bool

		loopEndIndex = len(txpkgs)
	)

	for i, txpkg := range txpkgs {
		newExecutedTxs, newAllLogs, newReceipts, gasLimitReached = e.ExecuteTxPackage(uint32(i), txpkg, prevStateDb, state, newReceipts, block, newExecutedTxs, usedGas, newAllLogs, gasPool, callbackParamKey)

		if gasLimitReached {
			loopEndIndex = i
			log.Warn("ExecuteTxPackages: reach block gas limit", "the last pkg index", loopEndIndex)
			break
		}
	}

	return newExecutedTxs, newAllLogs, newReceipts, loopEndIndex
}

func (e *WasmExecutor) ExecuteTxPackage(txPackageIndex uint32, txPackage *types.TxPackage, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, bool) {
	// package snapshot for revert
	oldState := state.Copy()
	oldGasPool := *gasPool
	oldUsedGas := *usedGas

	nonceSet := state.PackageNonceSet(txPackage.Packer())
	if nonceSet == nil {
		log.Error("cannot find package nonce set", "addr", txPackage.Packer())
		return nil, nil, nil, false
	}

	var (
		needReset     bool
		newStartNonce uint64
	)
	if prevStateDb != nil {
		needReset = true
		newStartNonce = prevStateDb.GetPackageNonce(txPackage.Packer())
	}

	searchResult, err, resetChanged := nonceSet.ResetThenSearch(needReset, newStartNonce, txPackage.Nonce(), e.maxBitLength)
	if err != nil {
		log.Error("ExecuteTxPackage error: ResetThenSearch Error", "err", err)
		return nil, nil, nil, false
	}

	if searchResult != nonces.NotContainedAndAllowed {
		log.Debug("ExecuteTxPackage error: NonceError", "searchResult", searchResult, "txPackage.Nonce", txPackage.Nonce(), "txPackage.Packer", txPackage.Packer())
		return nil, nil, nil, false
	}

	newExecutedTxs, newAllLogs, newReceipts, loopEndIndex := e.ExecuteTransactions(txPackage.Transactions(), prevStateDb, state, receipts, block, txPackageIndex, executedTxs, usedGas, allLogs, gasPool, callbackParamKey)

	if loopEndIndex < len(txPackage.Transactions()) {
		// revert
		state.ResetTo(oldState)
		*gasPool = oldGasPool
		*usedGas = oldUsedGas
		return executedTxs, allLogs, receipts, true
	}

	_, addChanged := nonceSet.Add(txPackage.Nonce(), e.maxBitLength)

	if resetChanged || addChanged {
		state.MarkPackageNonceSetJournal(txPackage.Packer())
		state.FinaliseOne()
	}

	log.Info("ExecuteTxPackage success", "txPackage.Nonce", txPackage.Nonce(), "txPackage.Packer", txPackage.Packer())
	return newExecutedTxs, newAllLogs, newReceipts, false
}

func (e *WasmExecutor) ExecuteTransactions(txs types.Transactions, prevStateDb *state.StateDB, state *state.StateDB, receipts types.Receipts, block *types.Block, txPackageIndex uint32, executedTxs []*types.TxWithIndex, usedGas *uint64, allLogs []*types.Log, gasPool *types.GasPool, callbackParamKey uint64) ([]*types.TxWithIndex, []*types.Log, types.Receipts, int) {
	var (
		newExecutedTxs = executedTxs
		newAllLogs     = allLogs
		newReceipts    = receipts

		loopEndIndex = len(txs)
	)

	for i, tx := range txs {
		// Start executing the transaction
		state.Prepare(tx.Hash(), txPackageIndex, uint32(i))

		receipt, _, err := e.ExecuteTransaction(prevStateDb, state, tx, block, gasPool, usedGas, callbackParamKey)
		if err == nil {
			// Everything ok, collect the logs and shift in the next transaction from the same account
			newExecutedTxs = append(newExecutedTxs, &types.TxWithIndex{Tx: tx, TxPackageIndex: txPackageIndex, TxIndex: uint32(i)})
			newReceipts = append(newReceipts, receipt)
			newAllLogs = append(newAllLogs, receipt.Logs...)
		} else if err == types.ErrGasLimitReached {
			loopEndIndex = i
			log.Warn("ExecuteTransactions: reach block gas limit", "the last tx index", loopEndIndex)
			break
		}
	}
	log.Debug("ExecuteTransactions: exec transactions", "num", len(newExecutedTxs))

	return newExecutedTxs, newAllLogs, newReceipts, loopEndIndex
}

func (e *WasmExecutor) ExecuteTransaction(prevStateDb *state.StateDB, state *state.StateDB, tx *types.Transaction, block *types.Block, gp *types.GasPool, usedGas *uint64, callbackParamKey uint64) (*types.Receipt, common.Address, error) {
	snap := state.Snapshot()

	receipt, _, from, err := e.ApplyTransaction(prevStateDb, state, tx, block, gp, usedGas, e.maxBitLength, callbackParamKey)

	if err != nil {
		state.RevertToSnapshot(snap)
		return nil, common.Address{}, err
	}

	state.FinaliseOne()

	return receipt, from, nil
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
func (e *WasmExecutor) ApplyTransaction(prevStateDb *state.StateDB, state *state.StateDB, tx *types.Transaction, block *types.Block, gp *types.GasPool, usedGas *uint64, maxBitLength uint64, callbackParamKey uint64) (*types.Receipt, uint64, common.Address, error) {
	msg, err := tx.AsMessage(e.signer)
	if err != nil {
		return nil, 0, common.Address{}, err
	}
	//log.Info("Apply Transaction", "from", msg.From(), "to", msg.To(), "hash", tx.Hash(), "nonce", msg.Nonce(), "data", msg.Data())
	_, useGas, wasmFailed, err := WasmApplyMessage(prevStateDb, state, msg, gp, maxBitLength, block.Header.Coinbase, callbackParamKey)

	if err != nil {
		if wasmFailed {
			log.Warn("WASM execute failed", "from", msg.From(), "to", msg.To(), "nonce", msg.Nonce(), "hash", tx.Hash(), "err", err)
		} else {
			log.Info("ApplyTransaction err", "from", msg.From(), "to", msg.To(), "nonce", msg.Nonce(), "hash", tx.Hash(), "err", err)
			return nil, 0, common.Address{}, err
		}
	}
	*usedGas += useGas

	// Create a new receipt for the transaction, storing the logs and gas used.
	receipt := types.NewReceipt(nil, wasmFailed, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = useGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(msg.From(), msg.Nonce())
	}
	// Set the receipt logs and create a bloom for filtering
	receipt.Logs = state.GetLogs(tx.Hash())
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	return receipt, useGas, msg.From(), nil
}

func WasmApplyMessage(prevStateDb *state.StateDB, statedb *state.StateDB, msg Message, gp *types.GasPool, maxBitLength uint64, coinbase common.Address, callbackParamKey uint64) ([]byte, uint64, bool, error) {
	nonceSet := statedb.TxNonceSet(msg.From())
	if nonceSet == nil {
		log.Error("WasmApplyMessage: cannot find tx nonce set", "addr", msg.From())
		return nil, 0, false, ErrNonceSetNotFound
	}

	return NewStateTransition(prevStateDb, statedb, msg, gp, nonceSet, maxBitLength, callbackParamKey).WasmTransitionDb(coinbase)
}

func CallWasmContract(code []byte, action []byte, from common.Address, to common.Address, user common.Address, owner common.Address, amount uint64, storageDelegate bool, userDelegate bool, remainedGas *uint64, callbackParamKey uint64) int {
	codePointer := unsafe.Pointer(&code[0])
	codeLength := len(code)
	actionPointer := unsafe.Pointer(&action[0])
	actionLength := len(action)
	fromAddrPointer := unsafe.Pointer(&from[0])
	toAddrPointer := unsafe.Pointer(&to[0])
	userAddrPointer := unsafe.Pointer(&user[0])
	ownerPointer := unsafe.Pointer(&owner[0])

	// prepare
	wasm.GetGlobalRegisterParam().SetRemainedGas(callbackParamKey, remainedGas)
	wasm.GetGlobalRegisterParam().AddCallstack(callbackParamKey, from, to, user, action, storageDelegate, userDelegate)

	// check depth
	depth := wasm.GetGlobalRegisterParam().GetCurrentDepth(callbackParamKey)
	if depth >= MaxWasmCallDepth {
		wasm.GetGlobalRegisterParam().FulfillCallstack(callbackParamKey, wasm.WasmErrorDepthExceed, "wasm call depth exceed", 0)
		return wasm.WasmErrorDepthExceed
	}

	preGas := *remainedGas
	ret := int(C.execute_go((*C.uchar)(codePointer), C.int(codeLength), (*C.uchar)(actionPointer), C.int(actionLength), (*C.uchar)(fromAddrPointer), (*C.uchar)(toAddrPointer), (*C.uchar)(ownerPointer), (*C.uchar)(userAddrPointer), (C.ulonglong)(amount), (*C.ulonglong)(remainedGas), C.ulonglong(callbackParamKey)))
	postGas := *remainedGas
	gas := preGas - postGas
	wasm.GetGlobalRegisterParam().FulfillCallstack(callbackParamKey, ret, "", gas)
	log.Info("Call wasm contract finish", "depth", depth, "gas", gas, "ret", ret)
	return ret
}

func Pointer2Address(pointer unsafe.Pointer) (common.Address, error) {
	if pointer == nil {
		return common.Address{}, errors.New("address is nil")
	}

	var address common.Address
	addrSlice := (*[1 << 28]byte)(pointer)[:common.AddressLength:common.AddressLength]
	copy(address[:], addrSlice)
	return address, nil
}

func Pointer2Slice(pointer unsafe.Pointer, length int) ([]byte, error) {
	if pointer == nil {
		return []byte{}, errors.New("pointer is nil")
	}

	bytes := (*[1 << 28]byte)(pointer)[:length:length]
	return bytes, nil
}

//export c_db_store
func c_db_store(callbackParamKey C.ulonglong, table C.ulonglong, key *C.char, keyLength C.int, value *C.char, valueLength C.int) {
	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_store convert key failed", "err", err.Error())
		return
	}

	valueSlice, err := Pointer2Slice(unsafe.Pointer(value), int(valueLength))
	if err != nil {
		log.Error("c_db_store convert value failed", "err", err.Error())
		return
	}

	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	wasm.DbStore(uint64(callbackParamKey), storage, uint64(table), keySlice, valueSlice)
}

//export c_db_load
func c_db_load(callbackParamKey C.ulonglong, table C.ulonglong, key *C.char, keyLength C.int, value *C.char, valueLength C.int) C.int {
	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_load convert key failed", "err", err.Error())
		return -1
	}

	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	valueSlice := wasm.DbLoad(uint64(callbackParamKey), storage, uint64(table), keySlice)
	if value != nil && valueLength > 0 {
		targetSlice := (*[1 << 28]C.char)(unsafe.Pointer(value))[:int(valueLength):int(valueLength)]
		for i := 0; i < int(valueLength) && i < len(valueSlice); i++ {
			targetSlice[i] = C.char(valueSlice[i])
		}
	}
	return C.int(len(valueSlice))
}

//export c_db_has_key
func c_db_has_key(callbackParamKey C.ulonglong, table C.ulonglong, key *C.char, keyLength C.int) C.int {
	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_has_key convert key failed", "err", err.Error())
		return -1
	}

	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	return C.int(wasm.DbHasKey(uint64(callbackParamKey), storage, uint64(table), keySlice))
}

//export c_db_remove_key
func c_db_remove_key(callbackParamKey C.ulonglong, table C.ulonglong, key *C.char, keyLength C.int) {
	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_remove_key convert key failed", "err", err.Error())
		return
	}

	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	wasm.DbRemoveKey(uint64(callbackParamKey), storage, uint64(table), keySlice)
}

//export c_db_has_table
func c_db_has_table(callbackParamKey C.ulonglong, table C.ulonglong) C.int {
	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	return C.int(wasm.DbHasTable(uint64(callbackParamKey), storage, uint64(table)))
}

//export c_db_remove_table
func c_db_remove_table(callbackParamKey C.ulonglong, table C.ulonglong) {
	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	wasm.DbRemoveTable(uint64(callbackParamKey), storage, uint64(table))
}

//export c_chain_current_time
func c_chain_current_time(callbackParamKey C.ulonglong) C.ulonglong {
	return C.ulonglong(wasm.GetBlockRound(uint64(callbackParamKey)))
}

//export c_chain_current_height
func c_chain_current_height(callbackParamKey C.ulonglong) C.ulonglong {
	return C.ulonglong(wasm.GetBlockHeight(uint64(callbackParamKey)))
}

//export c_chain_current_hash
func c_chain_current_hash(callbackParamKey C.ulonglong, simpleHash *C.char) {
	sh := wasm.GetBlockHash(uint64(callbackParamKey))
	simpleHashSlice := (*[1 << 28]C.char)(unsafe.Pointer(simpleHash))[:common.HashLength:common.HashLength]
	for i := 0; i < common.HashLength; i++ {
		simpleHashSlice[i] = C.char(sh[i])
	}
}

//export c_add_log
func c_add_log(callbackParamKey C.ulonglong, topic *C.char, topicNum C.int, data *C.char, dataLength C.int) {
	topicSlice, err := Pointer2Slice(unsafe.Pointer(topic), int(topicNum)*common.HashLength)
	if err != nil {
		log.Error("c_add_log convert topic failed", "err", err.Error())
		return
	}

	dataSlice, err := Pointer2Slice(unsafe.Pointer(data), int(dataLength))
	if err != nil {
		log.Error("c_add_log convert data failed", "err", err.Error())
		return
	}

	storage := wasm.GetGlobalRegisterParam().GetCurrentStorage(uint64(callbackParamKey))
	wasm.AddLog(uint64(callbackParamKey), storage, topicSlice, int(topicNum), dataSlice, int(dataLength))
}

//export c_transfer
func c_transfer(callbackParamKey C.ulonglong, to *C.char, amount C.ulonglong) C.int {
	toAddr, err := Pointer2Address(unsafe.Pointer(to))
	if err != nil {
		log.Error("c_transfer convert to address failed", "err", err.Error())
		return -1
	}

	fromAddr := wasm.GetGlobalRegisterParam().GetCurrentContract(uint64(callbackParamKey))
	remainedGas := wasm.GetGlobalRegisterParam().GetRemainedGas(uint64(callbackParamKey))
	return C.int(wasm.Transfer(uint64(callbackParamKey), fromAddr, toAddr, uint64(amount), remainedGas))
}

//export c_call_action
func c_call_action(callbackParamKey C.ulonglong, contract *C.char, actionBytes *C.char, actionLength C.int, amount C.ulonglong, storageDelegate C.int, userDelegate C.int) C.int {
	contractAddr, err := Pointer2Address(unsafe.Pointer(contract))
	if err != nil {
		log.Error("c_call_action convert contract address failed", "err", err.Error())
		return -1
	}

	actionSlice, err := Pointer2Slice(unsafe.Pointer(actionBytes), int(actionLength))
	if err != nil {
		log.Error("c_call_action convert action failed", "err", err.Error())
		return -1
	}

	storageDelegate_ := true
	if int(storageDelegate) == 0 {
		storageDelegate_ = false
	}
	userDelegate_ := true
	if int(userDelegate) == 0 {
		userDelegate_ = false
	}

	code := wasm.GetGlobalRegisterParam().GetContractCode(uint64(callbackParamKey), contractAddr)
	if code == nil {
		log.Error("c_call_action call contract failed: contract code is nil")
	}
	owner := wasm.GetGlobalRegisterParam().GetContractOwner(uint64(callbackParamKey), contractAddr)
	from := wasm.GetGlobalRegisterParam().GetCurrentContract(uint64(callbackParamKey))
	user := wasm.GetGlobalRegisterParam().GetCurrentUser(uint64(callbackParamKey))
	remainedGas := wasm.GetGlobalRegisterParam().GetRemainedGas(uint64(callbackParamKey))
	ret := CallWasmContract(code, actionSlice, from, contractAddr, owner, user, uint64(amount), storageDelegate_, userDelegate_, remainedGas, uint64(callbackParamKey))
	return C.int(ret)
}

//export c_call_result
func c_call_result(callbackParamKey C.ulonglong, result *C.char, length C.int) C.int {
	resultSlice := wasm.GetGlobalRegisterParam().GetCallResult(uint64(callbackParamKey))
	if result != nil && length > 0 {
		targetSlice := (*[1 << 28]C.char)(unsafe.Pointer(result))[:int(length):int(length)]
		for i := 0; i < int(length) && i < len(resultSlice); i++ {
			targetSlice[i] = C.char(resultSlice[i])
		}
	}
	return C.int(len(resultSlice))
}

//export c_set_result
func c_set_result(callbackParamKey C.ulonglong, result *C.char, length C.int) C.int {
	log.Info("c_set_result", "length", length)
	resultSlice, err := Pointer2Slice(unsafe.Pointer(result), int(length))
	if err != nil {
		log.Error("c_set_result convert result failed", "err", err.Error())
		return -1
	}

	wasm.GetGlobalRegisterParam().SetCallResult(uint64(callbackParamKey), resultSlice)
	return C.int(len(resultSlice))
}

//export c_call_depth
func c_call_depth(callbackParamKey C.ulonglong) C.uchar {
	depth := wasm.GetGlobalRegisterParam().GetCurrentDepth(uint64(callbackParamKey))
	return C.uchar(depth)
}

//export c_sha256
func c_sha256(input *C.char, length C.int, hash *C.char) C.int {
	inputSlice, err := Pointer2Slice(unsafe.Pointer(input), int(length))
	if err != nil {
		log.Error("c_sha256 convert input failed", "err", err.Error())
		return -1
	}

	hashSlice := sha3.Sum256(inputSlice)
	if hash != nil {
		targetSlice := (*[1 << 28]C.char)(unsafe.Pointer(hash))[:32:32]
		for i := 0; i < 32 && i < len(hashSlice); i++ {
			targetSlice[i] = C.char(hashSlice[i])
		}
	}
	return 0
}
