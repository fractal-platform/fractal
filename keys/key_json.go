package keys

import "errors"

var (
	ErrDecrypt      = errors.New("could not decrypt key with given passphrase")
	ErrNoMatch      = errors.New("no key for given address or file")
	ErrNotSupport   = errors.New("this function is not support")
	ErrAddrNotMatch = errors.New("the address is not match the account")
	ErrEmptyAddr    = errors.New("the address is empty")
	ErrNoValidKey   = errors.New("there is no valid key for this round")
)

const (
	version = 1.0
)

type keyfileJSON struct {
	Version       int               `json:"version"`
	ID            string            `json:"id"`
	Address       string            `json:"address"`
	Crypto        cryptoJSON        `json:"crypto"`
	PackerCrypto  cryptoJSON        `json:"packerCrypto"`
	MinerCryptoes []minerCryptoJSON `json:"minerCryptoes"`
}

type minerCryptoJSON struct {
	MinerCrypto cryptoJSON `json:"minerCrypto"`
	Timestamp   int64      `json:"timestamp"`
}

type cryptoJSON struct {
	Cipher       string                 `json:"cipher"`
	CipherText   string                 `json:"ciphertext"`
	CipherParams cipherparamsJSON       `json:"cipherparams"`
	KDF          string                 `json:"kdf"`
	KDFParams    map[string]interface{} `json:"kdfparams"`
	MAC          string                 `json:"mac"`
}

type cipherparamsJSON struct {
	IV string `json:"iv"`
}
