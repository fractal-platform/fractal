package state

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/big"
	"sort"
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/trie"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
)

type revision struct {
	id           int
	journalIndex int
}

var (
	// emptyState is the known hash of an empty state trie entry.
	emptyState = crypto.Keccak256Hash(nil)

	// emptyCode is the known hash of the empty WASM bytecode.
	emptyCode = crypto.Keccak256Hash(nil)

	miningRewardValue    = big.NewInt(5 * 1e9)
	confirmRewardValue   = big.NewInt(1 * 1e9)
	confirmedRewardValue = big.NewInt(1 * 1e9)
)

// StateDBs within the fractal protocol are used to store anything
// within the merkle trie. StateDBs take care of caching and storing
// nested states.
type StateDB struct {
	db   Database
	trie Trie

	// This map holds 'live' objects, which will get modified while processing a state transition.
	stateObjects      map[common.Address]*stateObject
	stateObjectsDirty map[common.Address]struct{}

	// DB error.
	// State objects are used by the consensus core which are unable to
	// deal with database-level errors. Any error that occurs during a
	// database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, may be used in the next version.
	refund uint64

	thash    common.Hash
	pkgIndex uint32
	txIndex  uint32
	logs     map[common.Hash][]*types.Log
	logSize  uint32

	// Journal of state modifications. This is the backbone of
	// Snapshot and RevertToSnapshot.
	journal        *journal
	validRevisions []revision
	nextRevisionId int

	lock sync.Mutex
}

func (s *StateDB) PrintNonceSet() {
	log.Info("stateDB stateObjects", "len(stateObjects)", len(s.stateObjects))
	for addr, stat := range s.stateObjects {
		log.Info("stateDB nonceSet", "addr", addr.String(), "PackageNonceSet", stat.data.PackageNonceSet.String(), "TxNonceSet", stat.data.TxNonceSet.String())
	}
}

// Create a new state from a given trie.
func New(root common.Hash, db Database) (*StateDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}
	return &StateDB{
		db:                db,
		trie:              tr,
		stateObjects:      make(map[common.Address]*stateObject),
		stateObjectsDirty: make(map[common.Address]struct{}),
		logs:              make(map[common.Hash][]*types.Log),
		journal:           newJournal(),
	}, nil
}

// setError remembers the first non-nil error it is called with.
func (self *StateDB) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *StateDB) Error() error {
	return self.dbErr
}

// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (self *StateDB) Reset(root common.Hash) error {
	tr, err := self.db.OpenTrie(root)
	if err != nil {
		return err
	}
	self.trie = tr
	self.stateObjects = make(map[common.Address]*stateObject)
	self.stateObjectsDirty = make(map[common.Address]struct{})
	self.thash = common.Hash{}
	self.txIndex = 0
	self.logs = make(map[common.Hash][]*types.Log)
	self.logSize = 0
	self.clearJournalAndRefund()
	return nil
}

func (self *StateDB) MarkNonceSetJournal(address common.Address, prev *nonces.NonceSet) {
	self.journal.append(nonceSetChange{
		account: &address,
		prev:    nonces.NewNonceSet(prev),
	})
}

func (self *StateDB) MarkPackageNonceSetJournal(address common.Address) {
	self.journal.append(packageNonceSetChange{
		account: &address,
	})
}

func (self *StateDB) AddLog(log *types.Log) {
	self.journal.append(addLogChange{txhash: self.thash})

	log.TxHash = self.thash
	log.PkgIndex = self.pkgIndex
	log.TxIndex = self.txIndex
	log.Index = self.logSize
	self.logs[self.thash] = append(self.logs[self.thash], log)
	self.logSize++
}

func (self *StateDB) GetLogs(hash common.Hash) []*types.Log {
	return self.logs[hash]
}

func (self *StateDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range self.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

func (self *StateDB) AddRefund(gas uint64) {
	self.journal.append(refundChange{prev: self.refund})
	self.refund += gas
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (self *StateDB) Exist(addr common.Address) bool {
	return self.getStateObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the specification (balance = nonce = code = 0)
func (self *StateDB) Empty(addr common.Address) bool {
	so := self.getStateObject(addr)
	return so == nil || so.empty()
}

func (self *StateDB) DB() Database {
	return self.db
}

// Retrieve the balance from the given address or 0 if object not found
func (self *StateDB) GetBalance(addr common.Address) *big.Int {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Balance()
	}
	return common.Big0
}

func (self *StateDB) GetNonce(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Nonce()
	}
	return 0
}

func (self *StateDB) GetPackageNonce(addr common.Address) uint64 {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.PackageNonce()
	}
	return 0
}

func (self *StateDB) TxNonceSet(addr common.Address) *nonces.NonceSet {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.TxNonceSet()
	}
	return nil
}

func (self *StateDB) PackageNonceSet(addr common.Address) *nonces.NonceSet {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.PackageNonceSet()
	}
	return nil
}

func (self *StateDB) GetCode(addr common.Address) []byte {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.Code(self.db)
	}
	return nil
}

func (self *StateDB) GetContractOwner(addr common.Address) common.Address {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.ContractOwner()
	}
	return common.Address{}
}

func (self *StateDB) GetCodeSize(addr common.Address) int {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return 0
	}
	if stateObject.code != nil {
		return len(stateObject.code)
	}
	size, err := self.db.ContractCodeSize(stateObject.addrHash, common.BytesToHash(stateObject.CodeHash()))
	if err != nil {
		self.setError(err)
	}
	return size
}

func (self *StateDB) GetCodeHash(addr common.Address) common.Hash {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return common.Hash{}
	}
	return common.BytesToHash(stateObject.CodeHash())
}

func (self *StateDB) GetState(addr common.Address, key StorageKey) []byte {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.GetState(self.db, key)
	}
	return nil
}

func (self *StateDB) GetPackerInfo(index uint32) *types.PackerInfo {
	table, _ := utils.String2Uint64(params.PackerKeyContractInfoTable)
	indexByte := make([]byte, 4)
	binary.LittleEndian.PutUint32(indexByte, index)
	storageKey := GetStorageKey(table, indexByte)
	storageBytes := self.GetState(common.HexToAddress(params.PackerKeyContractAddr), storageKey)
	if storageBytes == nil {
		log.Error("GetPackerInfo error, cannot find the packer info", "storageKey", hexutil.Encode(storageKey.ToSlice()))
		return nil
	}
	var coinbase common.Address
	copy(coinbase[:], storageBytes[4:24])
	var packerPubKey types.PackerECPubKey
	copy(packerPubKey[:], storageBytes[25:90])
	info := &types.PackerInfo{
		PackerPubKey: packerPubKey,
		Coinbase:     coinbase,
		RpcAddress:   string(storageBytes[91:]),
	}
	return info
}

func (self *StateDB) GetPackerNumber() uint32 {
	table, _ := utils.String2Uint64(params.PackerKeyContractSizeTable)
	storageKey := GetStorageKey(table, []byte{0})
	storageBytes := self.GetState(common.HexToAddress(params.PackerKeyContractAddr), storageKey)
	if storageBytes == nil {
		log.Error("GetPackerNumber error, cannot find the packer size", "storageKey", hexutil.Encode(storageKey.ToSlice()))
		return 0
	}
	number := binary.LittleEndian.Uint32(storageBytes[1:])
	return number
}

// Database retrieves the low level database supporting the lower level trie ops.
func (self *StateDB) Database() Database {
	return self.db
}

// StorageTrie returns the storage trie of an account.
// The return value is a copy and is nil for non-existent accounts.
func (self *StateDB) StorageTrie(addr common.Address) Trie {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return nil
	}
	cpy := stateObject.deepCopy(self)
	return cpy.updateTrie(self.db)
}

func (self *StateDB) HasSuicided(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject != nil {
		return stateObject.suicided
	}
	return false
}

/*
 * SETTERS
 */

// AddBalance adds amount to the account associated with addr.
func (self *StateDB) AddBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.AddBalance(amount)
	}
}

// SubBalance subtracts amount from the account associated with addr.
func (self *StateDB) SubBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SubBalance(amount)
	}
}

func (self *StateDB) SetBalance(addr common.Address, amount *big.Int) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetBalance(amount)
	}
}

func (self *StateDB) SetCode(addr common.Address, code []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetCode(crypto.Keccak256Hash(code), code)
	}
}

func (self *StateDB) SetContractOwner(addr common.Address, owner common.Address) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetContractOwner(owner)
	}
}

func (self *StateDB) SetState(addr common.Address, key StorageKey, value []byte) {
	stateObject := self.GetOrNewStateObject(addr)
	if stateObject != nil {
		stateObject.SetState(self.db, key, value)
	}
}

// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (self *StateDB) Suicide(addr common.Address) bool {
	stateObject := self.getStateObject(addr)
	if stateObject == nil {
		return false
	}
	self.journal.append(suicideChange{
		account:     &addr,
		prev:        stateObject.suicided,
		prevbalance: new(big.Int).Set(stateObject.Balance()),
	})
	stateObject.markSuicided()
	stateObject.data.Balance = new(big.Int)

	return true
}

//
// Setting, updating & deleting state object methods.
//

// updateStateObject writes the given object to the trie.
func (self *StateDB) updateStateObject(stateObject *stateObject) {
	addr := stateObject.Address()
	data, err := rlp.EncodeToBytes(stateObject)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	self.setError(self.trie.TryUpdate(addr[:], data))
}

// deleteStateObject removes the given object from the state trie.
func (self *StateDB) deleteStateObject(stateObject *stateObject) {
	stateObject.deleted = true
	addr := stateObject.Address()
	self.setError(self.trie.TryDelete(addr[:]))
}

// Retrieve a state object given by the address. Returns nil if not found.
func (self *StateDB) getStateObject(addr common.Address) (stateObject *stateObject) {
	// Prefer 'live' objects.
	if obj := self.stateObjects[addr]; obj != nil {
		if obj.deleted {
			return nil
		}
		return obj
	}

	// Load the object from the database.
	enc, err := self.trie.TryGet(addr[:])
	if len(enc) == 0 {
		self.setError(err)
		return nil
	}
	var data Account
	if err := rlp.DecodeBytes(enc, &data); err != nil {
		log.Error("Failed to decode state object", "addr", addr, "err", err)
		return nil
	}
	// Insert into the live set.
	obj := newObject(self, addr, data)
	self.setStateObject(obj)
	return obj
}

func (self *StateDB) setStateObject(object *stateObject) {
	self.stateObjects[object.Address()] = object
}

// Retrieve a state object or create a new state object if nil.
func (self *StateDB) GetOrNewStateObject(addr common.Address) *stateObject {
	stateObject := self.getStateObject(addr)
	if stateObject == nil || stateObject.deleted {
		stateObject, _ = self.createObject(addr)
	}
	return stateObject
}

// createObject creates a new state object. If there is an existing account with
// the given address, it is overwritten and returned as the second return value.
func (self *StateDB) createObject(addr common.Address) (newobj, prev *stateObject) {
	prev = self.getStateObject(addr)
	newobj = newObject(self, addr, Account{})
	if prev == nil {
		self.journal.append(createObjectChange{account: &addr})
	} else {
		self.journal.append(resetObjectChange{prev: prev})
	}
	self.setStateObject(newobj)
	return newobj, prev
}

// CreateAccount explicitly creates a state object. If a state object with the address
// already exists the balance is carried over to the new account.
//
// CreateAccount is called during the WASM CREATE operation. The new contract address is
// generated by the crypto.CreateAddress function.
//
// Carrying over the balance ensures that Balance doesn't disappear.
func (self *StateDB) CreateAccount(addr common.Address) {
	new, prev := self.createObject(addr)
	if prev != nil {
		new.setBalance(prev.data.Balance)
	}
}

// Copy creates a deep, independent copy of the state.
// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) Copy() *StateDB {
	self.lock.Lock()
	defer self.lock.Unlock()

	// Copy all the basic fields, initialize the memory ones
	state := &StateDB{
		db:                self.db,
		trie:              self.db.CopyTrie(self.trie),
		stateObjects:      make(map[common.Address]*stateObject, len(self.journal.dirties)),
		stateObjectsDirty: make(map[common.Address]struct{}, len(self.journal.dirties)),
		refund:            self.refund,
		logs:              make(map[common.Hash][]*types.Log, len(self.logs)),
		logSize:           self.logSize,
		journal:           newJournal(),
	}
	// Copy the dirty states, logs
	for addr := range self.journal.dirties {
		// there is a case where an object is in the journal but not
		// in the stateObjects. Thus, we need to check for nil
		if object, exist := self.stateObjects[addr]; exist {
			state.stateObjects[addr] = object.deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range self.stateObjectsDirty {
		if _, exist := state.stateObjects[addr]; !exist {
			state.stateObjects[addr] = self.stateObjects[addr].deepCopy(state)
			state.stateObjectsDirty[addr] = struct{}{}
		}
	}

	for hash, logs := range self.logs {
		state.logs[hash] = make([]*types.Log, len(logs))
		copy(state.logs[hash], logs)
	}

	return state
}

// Snapshots of the copied state cannot be applied to the copy.
func (self *StateDB) ResetTo(source *StateDB) {
	self.lock.Lock()
	defer self.lock.Unlock()

	self.db = source.db
	self.trie = source.db.CopyTrie(source.trie)
	self.stateObjects = make(map[common.Address]*stateObject, len(source.journal.dirties))
	self.stateObjectsDirty = make(map[common.Address]struct{}, len(source.journal.dirties))
	self.dbErr = source.dbErr
	self.refund = source.refund

	self.thash = source.thash
	self.pkgIndex = source.pkgIndex
	self.txIndex = source.txIndex
	self.logs = make(map[common.Hash][]*types.Log, len(source.logs))

	self.journal = newJournal()
	self.validRevisions = nil
	self.nextRevisionId = 0

	// Copy the dirty states, logs
	for addr := range source.journal.dirties {
		// there is a case where an object is in the journal but not
		// in the stateObjects. Thus, we need to check for nil
		if object, exist := source.stateObjects[addr]; exist {
			self.stateObjects[addr] = object.deepCopy(self)
			self.stateObjectsDirty[addr] = struct{}{}
		}
	}
	// Above, we don't copy the actual journal. This means that if the copy is copied, the
	// loop above will be a no-op, since the copy's journal is empty.
	// Thus, here we iterate over stateObjects, to enable copies of copies
	for addr := range source.stateObjectsDirty {
		if _, exist := self.stateObjects[addr]; !exist {
			self.stateObjects[addr] = source.stateObjects[addr].deepCopy(self)
			self.stateObjectsDirty[addr] = struct{}{}
		}
	}

	for hash, logs := range source.logs {
		self.logs[hash] = make([]*types.Log, len(logs))
		copy(self.logs[hash], logs)
	}
}

// Snapshot returns an identifier for the current revision of the state.
func (self *StateDB) Snapshot() int {
	id := self.nextRevisionId
	self.nextRevisionId++
	self.validRevisions = append(self.validRevisions, revision{id, self.journal.length()})
	return id
}

// RevertToSnapshot reverts all state changes made since the given revision.
func (self *StateDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(self.validRevisions), func(i int) bool {
		return self.validRevisions[i].id >= revid
	})
	if idx == len(self.validRevisions) || self.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := self.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	self.journal.revert(self, snapshot)
	self.validRevisions = self.validRevisions[:idx]
}

// GetRefund returns the current value of the refund counter.
func (self *StateDB) GetRefund() uint64 {
	return self.refund
}

// FinaliseOne is called since a complete transaction/package ends
func (s *StateDB) FinaliseOne() {
	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}

	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// Finalise finalises the state by removing the self destructed objects
// and clears the journal as well as the refunds.
func (s *StateDB) Finalise(deleteEmptyObjects bool) {
	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}

	for addr := range s.stateObjectsDirty {
		stateObject, exist := s.stateObjects[addr]
		if !exist {
			continue
		}

		if stateObject.suicided || (deleteEmptyObjects && stateObject.empty()) {
			s.deleteStateObject(stateObject)
		} else {
			stateObject.updateRoot(s.db)
			s.updateStateObject(stateObject)
		}
	}

	// Invalidate journal because reverting across transactions is not allowed.
	s.clearJournalAndRefund()
}

// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (s *StateDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	s.Finalise(deleteEmptyObjects)
	return s.trie.Hash()
}

func (s *StateDB) GetRoot() common.Hash {
	return s.trie.Hash()
}

// Prepare sets the current transaction hash and index and block hash which is
// used when the WASM emits new state logs.
func (self *StateDB) Prepare(thash common.Hash, pi uint32, ti uint32) {
	self.thash = thash
	self.pkgIndex = pi
	self.txIndex = ti
}

func (s *StateDB) clearJournalAndRefund() {
	s.journal = newJournal()
	s.validRevisions = s.validRevisions[:0]
	s.refund = 0
}

// Commit writes the state to the underlying in-memory trie database.
func (s *StateDB) Commit(deleteEmptyObjects bool) (root common.Hash, err error) {
	defer s.clearJournalAndRefund()

	for addr := range s.journal.dirties {
		s.stateObjectsDirty[addr] = struct{}{}
	}
	// Commit objects to the trie.
	for addr, stateObject := range s.stateObjects {
		_, isDirty := s.stateObjectsDirty[addr]
		switch {
		case stateObject.suicided || (isDirty && deleteEmptyObjects && stateObject.empty()):
			// If the object has been removed, don't bother syncing it
			// and just mark it for deletion in the trie.
			s.deleteStateObject(stateObject)
		case isDirty:
			// Write any contract code associated with the state object
			if stateObject.code != nil && stateObject.dirtyCode {
				s.db.TrieDB().InsertBlob(common.BytesToHash(stateObject.CodeHash()), stateObject.code)
				stateObject.dirtyCode = false
			}
			// Write any storage changes in the state object to its storage trie.
			if err := stateObject.CommitTrie(s.db); err != nil {
				return common.Hash{}, err
			}
			// Update the object in the main account trie.
			s.updateStateObject(stateObject)
		}
		delete(s.stateObjectsDirty, addr)
	}
	// Write trie changes.
	root, err = s.trie.Commit(func(leaf []byte, parent common.Hash) error {
		var account Account
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		if account.Root != emptyState {
			s.db.TrieDB().Reference(account.Root, parent)
		}
		code := common.BytesToHash(account.CodeHash)
		if code != emptyCode {
			s.db.TrieDB().Reference(code, parent)
		}
		return nil
	})
	log.Debug("Trie cache stats after commit", "misses", trie.CacheMisses(), "unloads", trie.CacheUnloads())
	return root, err
}

func (s *StateDB) DumpState(msg string) {
	for k, v := range s.stateObjects {
		log.Debug(msg, "hash", s.trie.Hash(), "addr", k, "balance", v.Balance())
	}
}

func (s *StateDB) DumpAllState(path string) {
	s.lock.Lock()

	tr := s.trie
	it := trie.NewIterator(tr.NodeIterator(nil))
	var accountMap = make(map[common.Address]AccountForStorage)
	for it.Next() {
		addr := common.BytesToAddress(tr.GetKey(it.Key))
		obj := s.getStateObject(addr)
		if obj == nil {
			log.Warn("DumpAllState: not an valid address", "addr", addr, "hashKey", it.Key)
			continue
		}
		var account AccountForStorage
		account.Code = ""
		if len(obj.Code(s.db)) > 0 {
			account.Code = hexutil.Encode(obj.Code(s.db))
		}

		account.Balance = obj.Balance()
		account.Owner = obj.ContractOwner()
		account.Storage = make(map[StorageKey][]byte)

		storageTrie := obj.getTrie(s.db)
		if storageTrie != nil {
			storeIt := trie.NewIterator(storageTrie.NodeIterator(nil))
			for storeIt.Next() {
				key := storageTrie.GetKey(storeIt.Key)
				enc, err := storageTrie.TryGet(key)
				if err == nil && len(enc) > 0 {
					_, content, _, err := rlp.Split(enc)
					if err == nil {
						account.Storage[LoadStorageKey(key)] = content
					}
				}
			}
		}

		accountMap[addr] = account
	}

	s.lock.Unlock()

	data, _ := json.MarshalIndent(accountMap, "", "    ")
	ioutil.WriteFile(path, data, 0644)
}

func MiningReward(state *StateDB, block *types.Block) {
	// TODO: Delete log comment
	//log.Info("mine reward", "coinbase", block.Header.Coinbase, "preBalance", state.GetBalance(block.Header.Coinbase))
	addr := block.Header.Coinbase
	state.AddBalance(addr, miningRewardValue)
}

func ConfirmReward(state *StateDB, block *types.Block, confirmedBlock *types.Block) {
	// TODO: Delete log comment
	//log.Info("confirm reward", "coinbase", block.Header.Coinbase, "preBalance", state.GetBalance(block.Header.Coinbase))
	//log.Info("confirmed reward", "coinbase", confirmedBlock.Header.Coinbase, "preBalance", state.GetBalance(confirmedBlock.Header.Coinbase))
	state.AddBalance(block.Header.Coinbase, confirmRewardValue)
	state.AddBalance(confirmedBlock.Header.Coinbase, confirmedRewardValue)
}
