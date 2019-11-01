package common

import (
	"github.com/fractal-platform/fractal/crypto/sha3"
	"github.com/fractal-platform/fractal/rlp"
)

func RlpHash(x interface{}) (h Hash) {
	hw := sha3.NewKeccak256()
	rlp.Encode(hw, x)
	hw.Sum(h[:0])
	return h
}
