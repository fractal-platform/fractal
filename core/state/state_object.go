package state

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/nonces"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/trie"
	"github.com/fractal-platform/fractal/utils"
)

var emptyCodeHash = crypto.Keccak256(nil)

type Code []byte

func (self Code) String() string {
	return string(self) //strings.Join(Disassemble(self), " ")
}

const MaxStorageKeyLength = 40

type StorageKey struct {
	Length uint8
	Bytes  [MaxStorageKeyLength]byte
}

func (self *StorageKey) ToSlice() []byte {
	return self.Bytes[:self.Length]
}

func LoadStorageKey(key []byte) StorageKey {
	var storageKey StorageKey
	storageKey.Length = uint8(utils.MinOf(len(key), MaxStorageKeyLength))
	copy(storageKey.Bytes[:], key)
	return storageKey
}

func GetStorageKey(table uint64, key []byte) StorageKey {
	var storageKey StorageKey

	// cut to MaxStorageKeyLength if exceeds
	storageKey.Length = uint8(utils.MinOf(8+len(key), MaxStorageKeyLength))

	binary.BigEndian.PutUint64(storageKey.Bytes[0:8], table)
	copy(storageKey.Bytes[8:], key)
	return storageKey
}

type Storage map[StorageKey][]byte

type StorageFormat struct {
	Key   string
	Value string
}

func (self Storage) toStorageFormats() []StorageFormat {
	ret := make([]StorageFormat, 0)
	for key, value := range self {
		ret = append(ret, StorageFormat{
			Key:   hexutil.Encode(key.ToSlice()),
			Value: hexutil.Encode(value[:]),
		})
	}
	return ret
}

func toStorage(storageFormats []StorageFormat) Storage {
	ret := make(Storage)
	for _, data := range storageFormats {
		key, _ := hexutil.Decode(data.Key)
		value, _ := hexutil.Decode(data.Value)
		storageKey := LoadStorageKey(key)
		ret[storageKey] = value
	}
	return ret
}

// EncodeRLP implements rlp.Encoder.
func (self Storage) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, self.toStorageFormats())
}

// DecodeRLP implements rlp.Decoder.
func (self *Storage) DecodeRLP(s *rlp.Stream) error {
	var storageFormats []StorageFormat
	if err := s.Decode((*[]StorageFormat)(&storageFormats)); err != nil {
		return err
	}
	*self = toStorage(storageFormats)
	return nil
}

func (self Storage) MarshalJSON() ([]byte, error) {
	return json.Marshal(self.toStorageFormats())
}

func (self *Storage) UnmarshalJSON(input []byte) error {
	var storageFormats []StorageFormat
	if err := json.Unmarshal(input, &storageFormats); err != nil {
		return err
	}
	*self = toStorage(storageFormats)
	return nil
}

func (self Storage) String() (str string) {
	for key, value := range self {
		str += fmt.Sprintf("%X : %X\n", key, value)
	}
	return
}

func (self Storage) Copy() Storage {
	cpy := make(Storage)
	for key, value := range self {
		cpy[key] = CopyStorageValue(value)
	}

	return cpy
}

func CopyStorageValue(value []byte) []byte {
	if value == nil {
		return nil
	}

	ret := make([]byte, len(value))
	copy(ret, value)
	return ret
}

// stateObject represents an Fractal account which is being modified.
//
// The usage pattern is as follows:
// First you need to obtain a state object.
// Account values can be accessed and modified through the object.
// Finally, call CommitTrie to write the modified storage trie into a database.
type stateObject struct {
	address  common.Address
	addrHash common.Hash // hash of fractal address of the account
	data     Account
	db       *StateDB

	// DB error.
	// State objects are used by the consensus core which are unable to
	// deal with database-level errors. Any error that occurs during a
	// database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// Write caches.
	trie Trie // storage trie, which becomes non-nil on first access
	code Code // contract bytecode, which gets set when code is loaded

	cachedStorage Storage // Storage entry cache to avoid duplicate reads
	dirtyStorage  Storage // Storage entries that need to be flushed to disk

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	dirtyCode bool
	suicided  bool
	deleted   bool
}

// empty returns whether the account is considered empty.
func (s *stateObject) empty() bool {
	return (&s.data.TxNonceSet).NextNonce() == 0 && (&s.data.PackageNonceSet).NextNonce() == 0 && s.data.Balance.Sign() == 0 && bytes.Equal(s.data.CodeHash, emptyCodeHash)
}

// Account is the Fractal consensus representation of accounts.
// These objects are stored in the main account trie.
type Account struct {
	TxNonceSet      nonces.NonceSet
	PackageNonceSet nonces.NonceSet

	Balance       *big.Int
	LockedBalance *big.Int
	LockToRound   uint64 // time uint(100ms): same to the "Round" in block

	Root     common.Hash
	CodeHash []byte

	ContractOwner common.Address
}

type AccountForStorage struct {
	Code          string         `json:"code,omitempty"`
	Storage       Storage        `json:"storage,omitempty"`
	Balance       *big.Int       `json:"balance" gencodec:"required"`
	LockedBalance *big.Int       `json:"lockedBalance" gencodec:"required"`
	LockToRound   uint64         `json:"lockToRound" gencodec:"required"`
	PrivateKey    []byte         `json:"secretKey,omitempty"` // for tests
	Owner         common.Address `json:"owner,omitempty"`
}

// newObject creates a state object.
func newObject(db *StateDB, address common.Address, data Account) *stateObject {
	tmpData := data
	if data.Balance == nil {
		tmpData.Balance = new(big.Int)
	}
	if data.LockedBalance == nil {
		tmpData.LockedBalance = new(big.Int)
	}
	if data.CodeHash == nil {
		tmpData.CodeHash = emptyCodeHash
	}
	if data.TxNonceSet.BitMask == nil {
		tmpData.TxNonceSet.BitMask = make(nonces.NonceBitMask, 0)
	} else {
		tmpData.TxNonceSet.BitMask = make(nonces.NonceBitMask, len(data.TxNonceSet.BitMask))
		copy(tmpData.TxNonceSet.BitMask, data.TxNonceSet.BitMask)
	}
	if data.PackageNonceSet.BitMask == nil {
		tmpData.PackageNonceSet.BitMask = make(nonces.NonceBitMask, 0)
	} else {
		tmpData.PackageNonceSet.BitMask = make(nonces.NonceBitMask, len(data.PackageNonceSet.BitMask))
		copy(tmpData.PackageNonceSet.BitMask, data.PackageNonceSet.BitMask)
	}
	return &stateObject{
		db:            db,
		address:       address,
		addrHash:      crypto.Keccak256Hash(address[:]),
		data:          tmpData,
		cachedStorage: make(Storage),
		dirtyStorage:  make(Storage),
	}
}

// EncodeRLP implements rlp.Encoder.
func (c *stateObject) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, c.data)
}

// setError remembers the first non-nil error it is called with.
func (self *stateObject) setError(err error) {
	if self.dbErr == nil {
		self.dbErr = err
	}
}

func (self *stateObject) markSuicided() {
	self.suicided = true
}

func (c *stateObject) touch() {
	c.db.journal.append(touchChange{
		account: &c.address,
	})
	if c.address == ripemd {
		// Explicitly put it in the dirty-cache, which is otherwise generated from
		// flattened journals.
		c.db.journal.dirty(c.address)
	}
}

func (c *stateObject) getTrie(db Database) Trie {
	if c.trie == nil {
		var err error
		c.trie, err = db.OpenStorageTrie(c.addrHash, c.data.Root)
		if err != nil {
			c.trie, _ = db.OpenStorageTrie(c.addrHash, common.Hash{})
			c.setError(fmt.Errorf("can't create storage trie: %v", err))
		}
	}
	return c.trie
}

// GetState returns a value in account storage.
func (self *stateObject) GetState(db Database, key StorageKey) []byte {
	value, exists := self.cachedStorage[key]
	if exists {
		return CopyStorageValue(value)
	}
	// Load from DB in case it is missing.
	enc, err := self.getTrie(db).TryGet(key.ToSlice())
	if err != nil {
		self.setError(err)
		return nil
	}
	if len(enc) > 0 {
		_, content, _, err := rlp.Split(enc)
		if err != nil {
			self.setError(err)
			return nil
		}
		value = CopyStorageValue(content)
	}
	self.cachedStorage[key] = value
	return value
}

// SetState updates a value in account storage.
func (self *stateObject) SetState(db Database, key StorageKey, value []byte) {
	self.db.journal.append(storageChange{
		account:  &self.address,
		key:      key,
		prevalue: self.GetState(db, key),
	})
	self.setState(key, value)
}

func (self *stateObject) setState(key StorageKey, value []byte) {
	self.cachedStorage[key] = CopyStorageValue(value)
	self.dirtyStorage[key] = CopyStorageValue(value)
}

func (self *stateObject) HasKey(db Database, storageKey StorageKey) int {
	if self.GetState(db, storageKey) == nil {
		return 0
	}
	return 1
}

func (self *stateObject) HasTable(db Database, table uint64) int {
	tr := self.getTrie(db)
	it := trie.NewIterator(tr.NodeIterator(nil))
	for it.Next() {
		if binary.BigEndian.Uint64(tr.GetKey(it.Key)[0:8]) == table {
			return 1
		}
	}
	return 0
}

func (self *stateObject) GetKeysInTable(db Database, table uint64) []StorageKey {
	var keys []StorageKey
	tr := self.getTrie(db)
	it := trie.NewIterator(tr.NodeIterator(nil))
	for it.Next() {
		key := tr.GetKey(it.Key)
		if binary.BigEndian.Uint64(key[0:8]) == table {
			keys = append(keys, LoadStorageKey(key))
		}
	}
	return keys
}

func (self *stateObject) RemoveKey(db Database, storageKey StorageKey) {
	self.SetState(db, storageKey, nil)
}

func (self *stateObject) RemoveTable(db Database, table uint64) {
	var removeKey []StorageKey
	tr := self.getTrie(db)
	it := trie.NewIterator(tr.NodeIterator(nil))
	for it.Next() {
		Key := tr.GetKey(it.Key)
		if binary.BigEndian.Uint64(Key[0:8]) == table {
			tmp := LoadStorageKey(Key)
			removeKey = append(removeKey, tmp)
		}
	}

	for _, removeKey := range removeKey {
		self.SetState(db, removeKey, nil)
	}
}

// updateTrie writes cached storage modifications into the object's storage trie.
func (self *stateObject) updateTrie(db Database) Trie {
	tr := self.getTrie(db)
	for key, value := range self.dirtyStorage {
		delete(self.dirtyStorage, key)
		if value == nil {
			self.setError(tr.TryDelete(key.ToSlice()))
			continue
		}
		// Encoding []byte cannot fail, ok to ignore the error.
		v, _ := rlp.EncodeToBytes(value)
		self.setError(tr.TryUpdate(key.ToSlice(), v))
	}
	return tr
}

// UpdateRoot sets the trie root to the current root hash of
func (self *stateObject) updateRoot(db Database) {
	self.updateTrie(db)
	self.data.Root = self.trie.Hash()
}

// CommitTrie the storage trie of the object to db.
// This updates the trie root.
func (self *stateObject) CommitTrie(db Database) error {
	self.updateTrie(db)
	if self.dbErr != nil {
		return self.dbErr
	}
	root, err := self.trie.Commit(nil)
	if err == nil {
		self.data.Root = root
	}
	return err
}

// AddBalance removes amount from c's balance.
// It is used to add funds to the destination account of a transfer.
func (c *stateObject) AddBalance(amount *big.Int) {
	// We must check emptiness for the objects such that the account
	// clearing (0,0,0 objects) can take effect.
	if amount.Sign() == 0 {
		if c.empty() {
			c.touch()
		}

		return
	}
	c.SetBalance(new(big.Int).Add(c.Balance(), amount))
}

// SubBalance removes amount from c's balance.
// It is used to remove funds from the origin account of a transfer.
func (c *stateObject) SubBalance(amount *big.Int) {
	if amount.Sign() == 0 {
		return
	}
	c.SetBalance(new(big.Int).Sub(c.Balance(), amount))
}

func (self *stateObject) SetBalance(amount *big.Int) {
	self.db.journal.append(balanceChange{
		account: &self.address,
		prev:    new(big.Int).Set(self.data.Balance),
	})
	self.setBalance(amount)
}

func (self *stateObject) setBalance(amount *big.Int) {
	self.data.Balance = amount
}

// only set in genesis block when init
func (self *stateObject) setBalanceLock(amount *big.Int, lockToRound uint64) {
	// The locked balance cannot exceed the total balance
	if amount == nil {
		amount = common.Big0
	}
	if amount.Cmp(self.data.Balance) > 0 {
		amount = new(big.Int).Set(self.data.Balance)
	}
	self.data.LockedBalance = amount
	self.data.LockToRound = lockToRound
}

func (self *stateObject) deepCopy(db *StateDB) *stateObject {
	stateObject := newObject(db, self.address, self.data)
	if self.trie != nil {
		stateObject.trie = db.db.CopyTrie(self.trie)
	}
	stateObject.code = self.code
	stateObject.dirtyStorage = self.dirtyStorage.Copy()
	stateObject.cachedStorage = self.dirtyStorage.Copy()
	stateObject.suicided = self.suicided
	stateObject.dirtyCode = self.dirtyCode
	stateObject.deleted = self.deleted
	return stateObject
}

//
// Attribute accessors
//

// Returns the address of the contract/account
func (c *stateObject) Address() common.Address {
	return c.address
}

// Code returns the contract code associated with this object, if any.
func (self *stateObject) Code(db Database) []byte {
	if self.code != nil {
		return self.code
	}
	if bytes.Equal(self.CodeHash(), emptyCodeHash) {
		return nil
	}
	code, err := db.ContractCode(self.addrHash, common.BytesToHash(self.CodeHash()))
	if err != nil {
		self.setError(fmt.Errorf("can't load code hash %x: %v", self.CodeHash(), err))
	}
	self.code = code
	return code
}

func (self *stateObject) SetCode(codeHash common.Hash, code []byte) {
	prevcode := self.Code(self.db.db)
	self.db.journal.append(codeChange{
		account:  &self.address,
		prevhash: self.CodeHash(),
		prevcode: prevcode,
	})
	self.setCode(codeHash, code)
}

func (self *stateObject) setCode(codeHash common.Hash, code []byte) {
	self.code = code
	self.data.CodeHash = codeHash[:]
	self.dirtyCode = true
}

func (self *stateObject) ContractOwner() common.Address {
	return self.data.ContractOwner
}

func (self *stateObject) SetContractOwner(owner common.Address) {
	self.db.journal.append(contractOwnerChange{
		account:   &self.address,
		prevOwner: self.data.ContractOwner,
	})
	self.setContractOwner(owner)
}

func (self *stateObject) setContractOwner(owner common.Address) {
	self.data.ContractOwner = owner
}

func (self *stateObject) CodeHash() []byte {
	return self.data.CodeHash
}

func (self *stateObject) Balance() *big.Int {
	return self.data.Balance
}

func (self *stateObject) LockedBalance() *big.Int {
	return self.data.LockedBalance
}

func (self *stateObject) LockToRound() uint64 {
	return self.data.LockToRound
}

func (self *stateObject) Nonce() uint64 {
	return (&self.data.TxNonceSet).NextNonce()
}

func (self *stateObject) PackageNonce() uint64 {
	return (&self.data.PackageNonceSet).NextNonce()
}

func (self *stateObject) TxNonceSet() *nonces.NonceSet {
	return &self.data.TxNonceSet
}

func (self *stateObject) PackageNonceSet() *nonces.NonceSet {
	return &self.data.PackageNonceSet
}

func (self *stateObject) setTxNonceSet(nonceSet *nonces.NonceSet) {
	(&self.data.TxNonceSet).DeepCopy(nonceSet)
}

//func (self *stateObject) setPackageNonceSet(packageNonceSet *nonces.NonceSet) {
//	(&self.data.PackageNonceSet).DeepCopy(packageNonceSet)
//}
