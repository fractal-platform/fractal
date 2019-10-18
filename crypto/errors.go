package crypto

import "errors"

var ErrKeyTypeNotSupport = errors.New("unsupported key type")

var ErrUnmarshalPubKey = errors.New("unknown error happened during unmarshal public key")

var errInvalidPubkey = errors.New("invalid secp256k1 public key")
