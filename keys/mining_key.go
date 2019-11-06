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
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/pborman/uuid"
)

const (
	scanInterval = time.Second * 10
)

type MiningKey struct {
	Address    common.Address
	PrivKey    crypto.PrivateKey
	CreateTime int64
}

type MiningPubkey [crypto.BlsPubkeyLen]byte

func PublicKeyForMining(key crypto.PublicKey) MiningPubkey {
	var k MiningPubkey
	copy(k[:], key.Marshal())
	return k
}

type MiningKeys map[MiningPubkey]*MiningKey

type MiningKeyManager struct {
	directory string
	password  string
	keys      map[common.Address]MiningKeys

	lock sync.RWMutex
	term chan struct{}
}

func NewMiningKeyManager(directory string, password string) *MiningKeyManager {
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		os.MkdirAll(directory, 0755)
	}

	return &MiningKeyManager{
		directory: directory,
		password:  password,
		keys:      make(map[common.Address]MiningKeys),
		term:      make(chan struct{}),
	}
}

func (s *MiningKeyManager) Start() {
	if s.Load() != nil {
		panic("unlock password error")
	}
	go func() {
		timer := time.NewTimer(scanInterval)
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

func (s *MiningKeyManager) Stop() {
	close(s.term)
}

func (s *MiningKeyManager) CreateKey(address common.Address) crypto.PublicKey {
	pub, pri, _ := crypto.NewKeys(crypto.BLS)

	kjson := new(keyfileJSON)
	kjson.Version = version
	kjson.ID = uuid.NewUUID().String()
	kjson.Address = hexutil.Encode(address[:])
	crypto, err := EncryptData(pri.Marshal(), []byte(s.password), scryptN, scryptP)
	if err != nil {
		log.Error("Encrypt data failed", "err", err.Error())
		return nil
	}
	kjson.MinerCryptoes = []minerCryptoJSON{
		{
			MinerCrypto: crypto,
			Timestamp:   time.Now().Unix(),
		},
	}

	path := filepath.Join(s.directory, fmt.Sprintf("%s.mk.json", kjson.ID))
	bytes, err := json.Marshal(kjson)
	if err != nil {
		log.Error("Marshal data failed", "err", err.Error())
		return nil
	}
	ioutil.WriteFile(path, bytes, 0644)

	return pub
}

func (s *MiningKeyManager) Load() error {
	// List all the files from the folder
	files, err := ioutil.ReadDir(s.directory)
	if err != nil {
		log.Error("Read folder failed", "path", s.directory, "err", err.Error())
		return err
	}

	var miningKeys []*MiningKey
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
		for _, minerCrypto := range keyJSON.MinerCryptoes {
			plainText, err := DecryptData(minerCrypto.MinerCrypto, s.password)
			if err != nil {
				log.Error("Decrypt Data failed", "path", path, "err", err.Error())
				continue
			}

			key, err := crypto.UnmarshalPrivKey(crypto.BLS, plainText)
			if err != nil {
				log.Error("Unmarshal Key failed", "path", path, "err", err.Error())
				continue
			}
			miningKeys = append(miningKeys, &MiningKey{
				Address:    address,
				PrivKey:    key,
				CreateTime: minerCrypto.Timestamp,
			})
		}
	}

	s.lock.Lock()
	defer s.lock.Unlock()
	for _, k := range miningKeys {
		address := k.Address
		if _, ok := s.keys[address]; !ok {
			s.keys[address] = make(map[MiningPubkey]*MiningKey)
		}
		keys := s.keys[address]
		pubkey := PublicKeyForMining(k.PrivKey.Public())
		keys[pubkey] = k
	}
	return nil
}

func (s *MiningKeyManager) Sign(address common.Address, pubkey MiningPubkey, hash []byte) ([]byte, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	keys, ok := s.keys[address]
	if !ok {
		return []byte{}, errors.New("key map not found")
	}
	key, ok := keys[pubkey]
	if !ok {
		return []byte{}, errors.New("key not found")
	}
	return key.PrivKey.Sign(hash)
}

func (s *MiningKeyManager) Keys() map[common.Address]MiningKeys {
	return s.keys
}
