package txexec

/*
#cgo CFLAGS: -I./
#cgo LDFLAGS: -L./ -lwasmlib
#include <stdio.h>
#include <stdlib.h>

void c_db_store(unsigned long long callbackParamKey, char *addr, unsigned long long table, char *key, int keyLength, char *value, int valueLength);
int c_db_load(unsigned long long callbackParamKey, char *addr, unsigned long long table, char *key, int keyLength, char *value, int valueLength);
int c_db_has_key(unsigned long long callbackParamKey, char *addr, unsigned long long table, char *key, int keyLength);
void c_db_remove_key(unsigned long long callbackParamKey, char *addr, unsigned long long table, char *key, int keyLength);
int c_db_has_table(unsigned long long callbackParamKey, char *addr, unsigned long long table);
void c_db_remove_table(unsigned long long callbackParamKey, char *addr, unsigned long long table);
unsigned long long c_chain_current_time(unsigned long long callbackParamKey);
unsigned long long c_chain_current_height(unsigned long long callbackParamKey);
void c_add_log(unsigned long long callbackParamKey, char *addr, char *data, int topicNum);
int c_transfer(unsigned long long callbackParamKey, char *from, char *to, unsigned long long amount, unsigned long long *remainedGas);

typedef struct {
	void *cb_store;
	void *cb_load;
	void *cb_has_key;
	void *cb_remove_key;
	void *cb_has_table;
	void *cb_remove_table;
	void *cb_current_time;
	void *cb_current_height;
	void *cb_add_log;
	void *c_transfer;
} Callbacks;

int execute(unsigned char *codeBytes, int codeLength, unsigned char *actionBytes, int actionLength, unsigned char *fromAddrBytes, unsigned char *toAddrBytes, unsigned char *owner, unsigned long long amount, unsigned long long *remainedGas, unsigned long long callbackParamKey, Callbacks *callbacks);

static inline int execute_go(unsigned char *codeBytes, int codeLength, unsigned char *actionBytes, int actionLength, unsigned char *fromAddrBytes, unsigned char *toAddrBytes, unsigned char *owner, unsigned long long amount, unsigned long long *remainedGas, unsigned long long callbackParamKey) {
	Callbacks callbacks= {	c_db_store, c_db_load, c_db_has_key, c_db_remove_key,
							c_db_has_table, c_db_remove_table, c_chain_current_time, c_chain_current_height,
							c_add_log, c_transfer };
	return execute(codeBytes, codeLength, actionBytes, actionLength, fromAddrBytes, toAddrBytes, owner, amount, remainedGas, callbackParamKey, &callbacks);
}
*/
import "C"
import (
	"errors"
	"unsafe"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/core/state"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/utils/log"
)

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

func CallWasmContract(codePointer unsafe.Pointer, codeLength int, actionPointer unsafe.Pointer, actionLength int, fromAddrPointer unsafe.Pointer, toAddrPointer unsafe.Pointer, ownerPointer unsafe.Pointer, amount uint64, remainedGas *uint64, callbackParamKey uint64) int {
	preGas := *remainedGas
	ret := int(C.execute_go((*C.uchar)(codePointer), C.int(codeLength), (*C.uchar)(actionPointer), C.int(actionLength), (*C.uchar)(fromAddrPointer), (*C.uchar)(toAddrPointer), (*C.uchar)(ownerPointer),
		(C.ulonglong)(amount), (*C.ulonglong)(remainedGas), C.ulonglong(callbackParamKey)))
	postGas := *remainedGas
	log.Info("call gas", "gas", preGas-postGas)
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
func c_db_store(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong, key *C.char, keyLength C.int, value *C.char, valueLength C.int) {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_store convert address failed", "err", err.Error())
		return
	}

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

	wasm.DbStore(uint64(callbackParamKey), addrSlice, uint64(table), keySlice, valueSlice)
}

//export c_db_load
func c_db_load(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong, key *C.char, keyLength C.int, value *C.char, valueLength C.int) C.int {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_load convert address failed", "err", err.Error())
		return -1
	}

	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_load convert key failed", "err", err.Error())
		return -1
	}

	valueSlice := wasm.DbLoad(uint64(callbackParamKey), addrSlice, uint64(table), keySlice)
	if value != nil && valueLength > 0 {
		targetSlice := (*[1 << 28]C.char)(unsafe.Pointer(value))[:int(valueLength):int(valueLength)]
		for i := 0; i < int(valueLength) && i < len(valueSlice); i++ {
			targetSlice[i] = C.char(valueSlice[i])
		}
	}
	return C.int(len(valueSlice))
}

//export c_db_has_key
func c_db_has_key(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong, key *C.char, keyLength C.int) C.int {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_has_key convert address failed", "err", err.Error())
		return -1
	}

	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_has_key convert key failed", "err", err.Error())
		return -1
	}

	return C.int(wasm.DbHasKey(uint64(callbackParamKey), addrSlice, uint64(table), keySlice))
}

//export c_db_remove_key
func c_db_remove_key(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong, key *C.char, keyLength C.int) {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_remove_key convert address failed", "err", err.Error())
		return
	}

	keySlice, err := Pointer2Slice(unsafe.Pointer(key), int(keyLength))
	if err != nil {
		log.Error("c_db_remove_key convert key failed", "err", err.Error())
		return
	}

	wasm.DbRemoveKey(uint64(callbackParamKey), addrSlice, uint64(table), keySlice)
}

//export c_db_has_table
func c_db_has_table(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong) C.int {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_has_table convert address failed", "err", err.Error())
		return -1
	}

	return C.int(wasm.DbHasTable(uint64(callbackParamKey), addrSlice, uint64(table)))
}

//export c_db_remove_table
func c_db_remove_table(callbackParamKey C.ulonglong, addr *C.char, table C.ulonglong) {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_db_has_table convert address failed", "err", err.Error())
		return
	}

	wasm.DbRemoveTable(uint64(callbackParamKey), addrSlice, uint64(table))
}

//export c_chain_current_time
func c_chain_current_time(callbackParamKey C.ulonglong) C.ulonglong {
	return C.ulonglong(wasm.GetBlockRound(uint64(callbackParamKey)))
}

//export c_chain_current_height
func c_chain_current_height(callbackParamKey C.ulonglong) C.ulonglong {
	return C.ulonglong(wasm.GetBlockHeight(uint64(callbackParamKey)))
}

//export c_add_log
func c_add_log(callbackParamKey C.ulonglong, addr *C.char, data *C.char, topicNum C.int) {
	addrSlice, err := Pointer2Address(unsafe.Pointer(addr))
	if err != nil {
		log.Error("c_add_log convert address failed", "err", err.Error())
		return
	}

	topicSlice, err := Pointer2Slice(unsafe.Pointer(data), int(topicNum)*common.HashLength)
	if err != nil {
		log.Error("c_add_log convert data failed", "err", err.Error())
		return
	}

	wasm.AddLog(uint64(callbackParamKey), addrSlice, topicSlice, int(topicNum))
}

//export c_transfer
func c_transfer(callbackParamKey C.ulonglong, from *C.char, to *C.char, amount C.ulonglong, remainedGas *C.ulonglong) C.int {
	fromAddr, err := Pointer2Address(unsafe.Pointer(from))
	if err != nil {
		log.Error("c_transfer convert from address failed", "err", err.Error())
		return -1
	}

	toAddr, err := Pointer2Address(unsafe.Pointer(to))
	if err != nil {
		log.Error("c_transfer convert to address failed", "err", err.Error())
		return -1
	}

	return C.int(wasm.Transfer(uint64(callbackParamKey), fromAddr, toAddr, uint64(amount), (*uint64)(remainedGas)))
}
