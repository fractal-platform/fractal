package keys

import (
	"encoding/json"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/utils/log"
	"github.com/pborman/uuid"
	"io/ioutil"
)

type AccountKey struct {
	Address common.Address
	PrivKey crypto.PrivateKey
}

func LoadAccountKey(path string, password string) (*AccountKey, error) {
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
	plainText, err := DecryptData(keyJSON.Crypto, password)
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
	return &AccountKey{
		Address: address,
		PrivKey: key,
	}, nil
}

func CreateAccountKey(keyfile string, password string) *AccountKey {
	pub, pri, _ := crypto.NewKeys(crypto.ECDSA)
	address := pub.ToAddress()

	var err error
	kjson := new(keyfileJSON)
	kjson.Version = version
	kjson.ID = uuid.NewUUID().String()
	kjson.Address = hexutil.Encode(address[:])
	kjson.Crypto, err = EncryptData(pri.Marshal(), []byte(password), scryptN, scryptP)
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

	return &AccountKey{
		Address: address,
		PrivKey: pri,
	}
}
