package state

import (
	"reflect"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

type account struct {
	nonce uint64
}

type ManagedState struct {
	*StateDB

	mu sync.RWMutex

	accounts map[common.Address]*account

	elemType reflect.Type
}

// ManagedState returns a new managed state with the statedb as it's backing layer
func ManageState(statedb *StateDB, elemType reflect.Type) *ManagedState {
	return &ManagedState{
		StateDB:  statedb.Copy(),
		accounts: make(map[common.Address]*account),
		elemType: elemType,
	}
}

// GetNonce returns the canonical nonce for the managed or unmanaged account.
//
// Because GetNonce mutates the DB, we must take a write lock.
func (ms *ManagedState) GetNonce(addr common.Address) uint64 {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	if account, ok := ms.accounts[addr]; !ok {
		if ms.elemType == reflect.TypeOf(types.TxPackage{}) {
			return ms.StateDB.GetPackageNonce(addr)
		} else {
			return ms.StateDB.GetNonce(addr)
		}
	} else {
		// Always make sure the state account nonce isn't actually higher
		// than the tracked one.
		so := ms.StateDB.getStateObject(addr)
		if ms.elemType == reflect.TypeOf(types.TxPackage{}) {
			if so != nil && account.nonce < so.PackageNonce() {
				// Should never happen
				log.Error("ManagedState GetNonce For TxPackage, the state account nonce is higher than the tracked one", "state nonce", so.PackageNonce(), "tracked nonce", account.nonce)
				ms.accounts[addr] = newAccount(so.PackageNonce())
			}
		} else {
			if so != nil && account.nonce < so.Nonce() {
				// Should never happen
				log.Error("ManagedState GetNonce For Tx, the state account nonce is higher than the tracked one", "state nonce", so.Nonce(), "tracked nonce", account.nonce)
				ms.accounts[addr] = newAccount(so.Nonce())
			}
		}
		return ms.accounts[addr].nonce
	}
}

// SetNonce sets the new canonical nonce for the managed state
func (ms *ManagedState) SetNonce(addr common.Address, nonce uint64) {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	ms.accounts[addr] = newAccount(nonce)
}

func newAccount(nonce uint64) *account {
	return &account{nonce}
}
