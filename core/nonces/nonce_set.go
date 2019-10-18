package nonces

import (
	"errors"
	"fmt"
	"github.com/fractal-platform/fractal/common"
)

var (
	ErrorNonceTooLowInNonceSet = errors.New("nonce too low in nonce set")
	ErrorStartNonce            = errors.New("start nonce error")
	ErrorNonceOverFlow         = errors.New("nonce over flow")
)

type SearchResult int8

const (
	NotAllowedTooNew       SearchResult = -1
	NotContainedAndAllowed SearchResult = 0
	Contained              SearchResult = 1
	NotAllowedTooOld       SearchResult = 2
)

type NonceBitMask []byte

type NonceSet struct {
	Start   uint64
	BitMask NonceBitMask
	Length  uint32 // num of bit
}

func (n *NonceSet) String() string {
	var res = "start:" + fmt.Sprintf("%d", n.Start) + "," + "bitmask:" + common.Bytes2Hex(n.BitMask) + "," + "length:" + fmt.Sprintf("%d", n.Length)
	return res
}

func NewNonceSet(other *NonceSet) *NonceSet {
	s := new(NonceSet)
	s.DeepCopy(other)
	return s
}

func (s *NonceSet) DeepCopy(other *NonceSet) {
	s.Start = other.Start
	s.BitMask = make(NonceBitMask, len(other.BitMask))
	copy(s.BitMask, other.BitMask)
	s.Length = other.Length
}

func (s *NonceSet) NextNonce() uint64 {
	return s.Start + uint64(s.Length)
}

func (s *NonceSet) Add(nonce uint64, maxLength uint64) (error, bool) {
	var changed = false

	if nonce < s.Start {
		return ErrorNonceTooLowInNonceSet, false
	}

	if nonce >= s.Start+maxLength {
		return ErrorNonceOverFlow, false
	}

	bitIndex := nonce - s.Start
	byteIndex := bitIndex / 8
	bitMask := byte(1) << byte(7-bitIndex%8)

	if byteIndex >= uint64(len(s.BitMask)) {
		s.BitMask = append(s.BitMask, make([]byte, byteIndex-uint64(len(s.BitMask))+1)...)
	}

	if s.BitMask[byteIndex]&bitMask == 0 {
		changed = true
	}
	s.BitMask[byteIndex] |= bitMask

	if nonce-s.Start+1 > uint64(s.Length) {
		s.Length = uint32(nonce - s.Start + 1)
	}

	return nil, changed
}

func (s *NonceSet) Search(nonce uint64, maxLength uint64) SearchResult {
	if nonce < s.Start {
		return NotAllowedTooOld
	}

	if nonce >= s.Start+maxLength {
		return NotAllowedTooNew
	}

	if nonce >= s.Start+uint64(s.Length) {
		return NotContainedAndAllowed
	}

	bitIndex := nonce - s.Start
	byteIndex := bitIndex / 8
	bitMask := byte(1) << byte(7-bitIndex%8)
	if s.BitMask[byteIndex]&bitMask > 0 {
		return Contained
	} else {
		return NotContainedAndAllowed
	}
}

func (s *NonceSet) Reset(start uint64) (error, bool) {
	if start < s.Start {
		return ErrorStartNonce, false
	}

	if start == s.Start {
		return nil, false
	}

	if start >= s.Start+uint64(s.Length) {
		s.Start = start
		s.BitMask = NonceBitMask{}
		s.Length = 0
		return nil, true
	}

	left := start - s.Start
	leftByte := int(left / 8)
	leftBit := byte(left % 8)

	s.Start = start
	s.Length = s.Length - uint32(left)

	// left shift
	s.BitMask = s.BitMask[leftByte:]
	for i := range s.BitMask {
		s.BitMask[i] = s.BitMask[i] << leftBit
		if i+1 < len(s.BitMask) {
			s.BitMask[i] |= s.BitMask[i+1] >> (byte(8) - leftBit)
		}
	}

	// remove right zero bytes
	if s.BitMask[len(s.BitMask)-1] == 0 {
		s.BitMask = s.BitMask[:len(s.BitMask)-1]
	}

	return nil, true
}

// if param 'needReset' is false , will not use param 'newStart'
func (s *NonceSet) ResetThenSearch(needReset bool, newStart uint64, nonce uint64, maxLength uint64) (SearchResult, error, bool) {
	var changed bool
	var err error
	if needReset {
		err, changed = s.Reset(newStart)
		if err != nil {
			return 0, err, changed
		}
	}

	searchResult := s.Search(nonce, maxLength)
	return searchResult, nil, changed
}
