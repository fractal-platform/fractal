// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
	"vuvuzela.io/crypto/bn256"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/math"
	"github.com/fractal-platform/fractal/crypto/sha3"
	"github.com/fractal-platform/fractal/rlp"
)

var (
	secp256k1N, _  = new(big.Int).SetString("fffffffffffffffffffffffffffffffebaaedce6af48a03bbfd25e8cd0364141", 16)
	secp256k1halfN = new(big.Int).Div(secp256k1N, big.NewInt(2))
)

// Keccak256 calculates and returns the Keccak256 hash of the input data.
func Keccak256(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// Keccak256Hash calculates and returns the Keccak256 hash of the input data,
// converting it to an internal Hash data structure.
func Keccak256Hash(data ...[]byte) (h common.Hash) {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}

// Keccak512 calculates and returns the Keccak512 hash of the input data.
func Keccak512(data ...[]byte) []byte {
	d := sha3.NewKeccak512()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

// CreateAddress creates an ethereum address given the bytes and the nonce
func CreateAddress(b common.Address, nonce uint64) common.Address {
	data, _ := rlp.EncodeToBytes([]interface{}{b, nonce})
	return common.BytesToAddress(Keccak256(data)[12:])
}

// CreateAddress2 creates an ethereum address given the address bytes, initial
// contract code and a salt.
func CreateAddress2(b common.Address, salt [32]byte, code []byte) common.Address {
	return common.BytesToAddress(Keccak256([]byte{0xff}, b.Bytes(), salt[:], Keccak256(code))[12:])
}

// ToECDSA creates a private key with the given D value.
func ToECDSA(d []byte) (*ecdsa.PrivateKey, error) {
	return toECDSA(d, true)
}

// ToECDSAUnsafe blindly converts a binary blob to a private key. It should almost
// never be used unless you are sure the input is valid and want to avoid hitting
// errors due to bad origin encoding (0 prefixes cut off).
func ToECDSAUnsafe(d []byte) *ecdsa.PrivateKey {
	priv, _ := toECDSA(d, false)
	return priv
}

// toECDSA creates a private key with the given D value. The strict parameter
// controls whether the key's length should be enforced at the curve size or
// it can also accept legacy encodings (0 prefixes).
func toECDSA(d []byte, strict bool) (*ecdsa.PrivateKey, error) {
	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	if strict && 8*len(d) != priv.Params().BitSize {
		return nil, fmt.Errorf("invalid length, need %d bits", priv.Params().BitSize)
	}
	priv.D = new(big.Int).SetBytes(d)

	// The priv.D must < N
	if priv.D.Cmp(secp256k1N) >= 0 {
		return nil, fmt.Errorf("invalid private key, >=N")
	}
	// The priv.D must not be zero or negative.
	if priv.D.Sign() <= 0 {
		return nil, fmt.Errorf("invalid private key, zero or negative")
	}

	priv.PublicKey.X, priv.PublicKey.Y = priv.PublicKey.Curve.ScalarBaseMult(d)
	if priv.PublicKey.X == nil {
		return nil, errors.New("invalid private key")
	}
	return priv, nil
}

// FromECDSA exports a private key into a binary dump.
func FromECDSA(priv *ecdsa.PrivateKey) []byte {
	if priv == nil {
		return nil
	}
	return math.PaddedBigBytes(priv.D, priv.Params().BitSize/8)
}

// UnmarshalPubkey converts bytes to a secp256k1 public key.
func UnmarshalPubkey(pub []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(S256(), pub)
	if x == nil {
		return nil, errInvalidPubkey
	}
	return &ecdsa.PublicKey{Curve: S256(), X: x, Y: y}, nil
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(S256(), pub.X, pub.Y)
}

// HexToECDSA parses a secp256k1 private key.
func HexToECDSA(hexkey string) (*ecdsa.PrivateKey, error) {
	b, err := hex.DecodeString(hexkey)
	if err != nil {
		return nil, errors.New("invalid hex string")
	}
	return ToECDSA(b)
}

// LoadECDSA loads a secp256k1 private key from the given file.
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	buf := make([]byte, 64)
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	if _, err := io.ReadFull(fd, buf); err != nil {
		return nil, err
	}

	key, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}
	return ToECDSA(key)
}

// SaveECDSA saves a secp256k1 private key to the given file with
// restrictive permissions. The key data is saved hex-encoded.
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	k := hex.EncodeToString(FromECDSA(key))
	return ioutil.WriteFile(file, []byte(k), 0600)
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), rand.Reader)
}

// ValidateSignatureValues verifies whether the signature values are valid with
// the given chain rules. The v value is assumed to be either 0 or 1.
func ValidateSignatureValues(v byte, r, s *big.Int, homestead bool) bool {
	if r.Cmp(common.Big1) < 0 || s.Cmp(common.Big1) < 0 {
		return false
	}
	// reject upper range of s values (ECDSA malleability)
	// see discussion in secp256k1/libsecp256k1/include/secp256k1.h
	if homestead && s.Cmp(secp256k1halfN) > 0 {
		return false
	}
	// Frontier: allow s to be in full N range
	return r.Cmp(secp256k1N) < 0 && s.Cmp(secp256k1N) < 0 && (v == 0 || v == 1)
}

func PubkeyToAddress(p ecdsa.PublicKey) common.Address {
	pubBytes := FromECDSAPub(&p)
	return common.BytesToAddress(Keccak256(pubBytes[1:])[12:])
}

func zeroBytes(bytes []byte) {
	for i := range bytes {
		bytes[i] = 0
	}
}

// -------------------------new crypto----------------

type PublicKey interface {
	Marshal() []byte
	Unmarshal(m []byte) error
	Verify(msg []byte, sig []byte) bool
	ToAddress() common.Address
}

type PrivateKey interface {
	Marshal() []byte
	Unmarshal(m []byte) error
	Public() PublicKey
	Sign(m []byte) ([]byte, error)
	ZeroKey() // set the secret data into zero.
}

type KeyType int

const (
	ECDSA KeyType = iota
	BLS
)

var g2gen = new(bn256.G2).ScalarBaseMult(big.NewInt(1))

type bn256PubKey struct {
	gx *bn256.G2
}

func (pk *bn256PubKey) Marshal() []byte {
	return pk.gx.Marshal()
}

func (pk *bn256PubKey) Unmarshal(m []byte) error {
	if pk.gx == nil {
		pk.gx = new(bn256.G2)
	}
	g2, ok := pk.gx.Unmarshal(m)
	if !ok {
		return ErrUnmarshalPubKey
	}
	pk.gx = g2
	return nil
}

func (pk *bn256PubKey) Verify(msg []byte, sig []byte) bool {
	hx, ok := new(bn256.G1).Unmarshal(sig)
	if !ok {
		return false
	}
	u := bn256.Pair(hx, g2gen)

	h := new(bn256.G1).HashToPoint(msg)
	p := bn256.Pair(h, pk.gx)

	return subtle.ConstantTimeCompare(u.Marshal(), p.Marshal()) == 1
}

func (sk *bn256PubKey) ToAddress() common.Address {
	return common.Address{}
}

const BlsPubkeyLen = 32 * 4

type bn256PriKey struct {
	bn256PubKey
	x *big.Int
}

func (sk *bn256PriKey) Marshal() []byte {
	pub := sk.bn256PubKey.Marshal()
	pri, _ := sk.x.MarshalText()
	out := make([]byte, BlsPubkeyLen+len(pri))
	copy(out, pub)
	copy(out[BlsPubkeyLen:], pri)
	return out
}

func (sk *bn256PriKey) Unmarshal(m []byte) (e error) {
	pub := m[:BlsPubkeyLen]
	pri := m[BlsPubkeyLen:]
	if e = sk.bn256PubKey.Unmarshal(pub); e != nil {
		return e
	}
	if sk.x == nil {
		sk.x = new(big.Int)
	}
	return sk.x.UnmarshalText(pri)
}

func (sk *bn256PriKey) Public() PublicKey {
	return &sk.bn256PubKey
}

func (sk *bn256PriKey) Sign(m []byte) (sig []byte, err error) {
	h := new(bn256.G1).HashToPoint(m)
	hx := new(bn256.G1).ScalarMult(h, sk.x)
	return hx.Marshal(), err
}

func (sk *bn256PriKey) ZeroKey() {
	bits := sk.x.Bits()
	for i := range bits {
		bits[i] = 0
	}
}

func (sk *bn256PriKey) ToAddress() common.Address {
	return common.Address{}
}

type ecdsaPubKey struct {
	key *ecdsa.PublicKey
}

func (pk *ecdsaPubKey) Marshal() []byte {
	pub := pk.key
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(S256(), pub.X, pub.Y)
}

func (pk *ecdsaPubKey) Unmarshal(m []byte) error {
	x, y := elliptic.Unmarshal(S256(), m)
	if x == nil {
		return errInvalidPubkey
	}
	pk.key = &ecdsa.PublicKey{Curve: S256(), X: x, Y: y}
	return nil
}

func (pk *ecdsaPubKey) Verify(msg []byte, sig []byte) bool {
	return VerifySignature(pk.Marshal(), msg, sig)
}

func (pk *ecdsaPubKey) ToAddress() common.Address {
	return PubkeyToAddress(*pk.key)
}

type ecdsaPriKey struct {
	key *ecdsa.PrivateKey
}

func (sk *ecdsaPriKey) Marshal() []byte {
	priv := sk.key
	return math.PaddedBigBytes(priv.D, priv.Params().BitSize/8)
}

func (sk *ecdsaPriKey) Unmarshal(d []byte) error {
	key, err := toECDSA(d, true)
	sk.key = key
	return err
}

func (sk *ecdsaPriKey) Public() PublicKey {
	return &ecdsaPubKey{&(sk.key.PublicKey)}
}

func (sk *ecdsaPriKey) Sign(m []byte) ([]byte, error) {
	return Sign(m, sk.key)
}

func (sk *ecdsaPriKey) ZeroKey() {
	bits := sk.key.D.Bits()
	for i := range bits {
		bits[i] = 0
	}
}

func NewKeys(kind KeyType) (pk PublicKey, sk PrivateKey, err error) {
	switch kind {
	case BLS:
		x, gx, err := bn256.RandomG2(rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		pk = &bn256PubKey{gx}
		sk = &bn256PriKey{bn256PubKey{gx}, x}
	case ECDSA:
		k, err := ecdsa.GenerateKey(S256(), rand.Reader)
		if err != nil {
			return nil, nil, err
		}
		pk = &ecdsaPubKey{key: &(k.PublicKey)}
		sk = &ecdsaPriKey{key: k}
	default:
		return nil, nil, ErrKeyTypeNotSupport
	}
	return pk, sk, err
}

func UnmarshalPubKey(kind KeyType, keyBytes []byte) (key PublicKey, err error) {
	switch kind {
	case ECDSA:
		key = new(ecdsaPubKey)
	case BLS:
		key = new(bn256PubKey)
	default:
		return nil, ErrKeyTypeNotSupport
	}
	err = key.Unmarshal(keyBytes)
	return key, err
}

func UnmarshalPrivKey(kind KeyType, keyBytes []byte) (key PrivateKey, err error) {
	switch kind {
	case ECDSA:
		key = new(ecdsaPriKey)
	case BLS:
		key = new(bn256PriKey)
	default:
		return nil, ErrKeyTypeNotSupport
	}
	err = key.Unmarshal(keyBytes)
	return key, err
}

func ECDSAPubKeyToAddress(key PublicKey) (common.Address) {
	pubKey := key.(*ecdsaPubKey)
	return PubkeyToAddress(*(pubKey.key))
}
