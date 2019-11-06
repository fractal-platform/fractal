package keys

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/pborman/uuid"
)

type PackerKey struct {
	Address common.Address
	PrivKey crypto.PrivateKey
}

func PublicKeyForPacker(key crypto.PublicKey) types.PackerECPubKey {
	var k types.PackerECPubKey
	copy(k[:], key.Marshal())
	return k
}

type PackerKeys map[types.PackerECPubKey]*PackerKey

type PackerKeyManager struct {
	directory string
	password  string
	keys      map[common.Address]PackerKeys

	lock sync.RWMutex
	term chan struct{}
	wg   sync.WaitGroup // for shutdown sync
}

func NewPackerKeyManager(directory string, password string) *PackerKeyManager {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.MkdirAll(directory, 0755)
	}

	return &PackerKeyManager{
		directory: directory,
		password:  password,
		keys:      make(map[common.Address]PackerKeys),
		term:      make(chan struct{}),
	}
}

func (s *PackerKeyManager) Start() {
	if s.Load() != nil {
		panic("unlock password error")
	}
	go func() {
		timer := time.NewTimer(scanInterval)
		s.wg.Add(1)
		defer s.wg.Done()
		for {
			select {
			case <-s.term:
				return
			case <-timer.C:
				if s.Load() != nil {
					panic("unlock password error")
				}
				timer.Reset(scanInterval)
			}
		}
	}()
}

func (s *PackerKeyManager) Stop() {
	close(s.term)
	s.wg.Wait()
}

func (s *PackerKeyManager) CreateKey(address common.Address) crypto.PublicKey {
	pub, pri, _ := crypto.NewKeys(crypto.ECDSA)

	var err error
	kjson := new(keyfileJSON)
	kjson.Version = version
	kjson.ID = uuid.NewUUID().String()
	kjson.Address = hexutil.Encode(address[:])
	kjson.PackerCrypto, err = EncryptData(pri.Marshal(), []byte(s.password), scryptN, scryptP)
	if err != nil {
		log.Error("Encrypt data failed", "err", err.Error())
		return nil
	}

	path := filepath.Join(s.directory, fmt.Sprintf("%s.pk.json", kjson.ID))
	bytes, err := json.Marshal(kjson)
	if err != nil {
		log.Error("Marshal data failed", "err", err.Error())
		return nil
	}
	ioutil.WriteFile(path, bytes, 0644)

	return pub
}

func (s *PackerKeyManager) Load() error {
	// List all the files from the folder
	files, err := ioutil.ReadDir(s.directory)
	if err != nil {
		log.Error("Read folder failed", "path", s.directory, "err", err.Error())
		return err
	}

	var packerKeys []*PackerKey
	for _, fi := range files {
		if fi.IsDir() {
			continue
		}
		if !strings.HasSuffix(fi.Name(), ".json") {
			continue
		}

		path := filepath.Join(s.directory, fi.Name())
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			log.Error("Read file failed", "path", path, "err", err.Error())
			continue
		}

		keyJSON := new(keyfileJSON)
		err = json.Unmarshal(bytes, &keyJSON)
		if err != nil {
			log.Error("Unmarshal file failed", "path", path, "err", err.Error())
			continue
		}

		address := common.HexToAddress(keyJSON.Address)
		plainText, err := DecryptData(keyJSON.PackerCrypto, s.password)
		if err != nil {
			log.Error("Decrypt data failed", "path", path, "err", err.Error())
			return err
		}
		log.Debug("Decrypt data", "plain", plainText)

		key, err := crypto.UnmarshalPrivKey(crypto.ECDSA, plainText)
		if err != nil {
			log.Error("Unmarshal data failed", "path", path, "err", err.Error())
			return err
		}

		packerKeys = append(packerKeys, &PackerKey{
			Address: address,
			PrivKey: key,
		})
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	for _, k := range packerKeys {
		address := k.Address
		if _, ok := s.keys[address]; !ok {
			s.keys[address] = make(map[types.PackerECPubKey]*PackerKey)
		}
		keys := s.keys[address]
		pubkey := PublicKeyForPacker(k.PrivKey.Public())
		keys[pubkey] = k
	}
	return nil
}

func (s *PackerKeyManager) GetPrivateKey(address common.Address, pubkey types.PackerECPubKey) (crypto.PrivateKey, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	keys, ok := s.keys[address]
	if !ok {
		return nil, errors.New("key map not found")
	}
	key, ok := keys[pubkey]
	if !ok {
		return nil, errors.New("key not found")
	}
	return key.PrivKey, nil
}

func (s *PackerKeyManager) Keys() map[common.Address]PackerKeys {
	return s.keys
}

func LoadPackerKey(path string, password string) (*PackerKey, error) {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Error("Read file failed", "path", path, "err", err.Error())
		return nil, err
	}

	keyJSON := new(keyfileJSON)
	err = json.Unmarshal(bytes, &keyJSON)
	if err != nil {
		log.Error("Parse file failed", "path", path, "err", err.Error())
		return nil, err
	}

	address := common.HexToAddress(keyJSON.Address)
	plainText, err := DecryptData(keyJSON.PackerCrypto, password)
	if err != nil {
		log.Error("Decrypt data failed", "path", path, "err", err.Error())
		return nil, err
	}
	log.Debug("Decrypt data", "plain", plainText)

	key, err := crypto.UnmarshalPrivKey(crypto.ECDSA, plainText)
	if err != nil {
		log.Error("Unmarshal data failed", "path", path, "err", err.Error())
		return nil, err
	}
	return &PackerKey{
		Address: address,
		PrivKey: key,
	}, nil
}

func CreatePackerKey(address common.Address, password string, keyfile string) crypto.PublicKey {
	pub, pri, _ := crypto.NewKeys(crypto.ECDSA)

	var err error
	kjson := new(keyfileJSON)
	kjson.Version = version
	kjson.ID = uuid.NewUUID().String()
	kjson.Address = hexutil.Encode(address[:])
	kjson.PackerCrypto, err = EncryptData(pri.Marshal(), []byte(password), scryptN, scryptP)
	if err != nil {
		log.Error("Encrypt data failed", "err", err.Error())
		return nil
	}

	bytes, err := json.Marshal(kjson)
	if err != nil {
		log.Error("Marshal data failed", "err", err.Error())
		return nil
	}
	ioutil.WriteFile(keyfile, bytes, 0644)

	return pub
}
