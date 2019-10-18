package bloomstorage

import (
	"errors"

	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
)

var (
	// errSectionOutOfBounds is returned if the user tried to add more bloom filters
	// to the batch than available space, or if tries to retrieve above the capacity.
	errSectionOutOfBounds = errors.New("section out of bounds")

	// errBloomBitOutOfBounds is returned if the user tried to retrieve specified
	// bit bloom above the capacity.
	errBloomBitOutOfBounds = errors.New("bloom bit out of bounds")
)

// Generator takes a number of bloom filters and generates the rotated bloom bits
// to be used for batched filtering.
type Generator struct {
	Blooms      [types.BloomBitLength][params.BloomByteSize]byte // Rotated Blooms for per-bit matching
	NextBloomId uint16                                           // Next section to set when adding a bloom
}

// NewGenerator creates a rotated bloom generator that can iteratively fill a
// batched bloom filter's bits.
func NewGenerator() *Generator {
	b := &Generator{}
	return b
}

// AddBloom takes a single bloom filter and sets the corresponding bit column
// in memory accordingly.
func (b *Generator) AddBloom(index uint16, bloom *types.Bloom) error {
	// Make sure we're not adding more bloom filters than our capacity
	if b.NextBloomId >= uint16(params.BloomBitsSize) {
		return errSectionOutOfBounds
	}
	if b.NextBloomId != index {
		return errors.New("bloom filter with unexpected index")
	}
	// Rotate the bloom and insert into our collection
	byteIndex := b.NextBloomId / 8
	bitMask := byte(1) << byte(7-b.NextBloomId&7)

	for i := 0; i < types.BloomBitLength; i++ {
		bloomByteIndex := types.BloomByteLength - 1 - i/8
		bloomBitMask := byte(1) << byte(i&7)

		if (bloom[bloomByteIndex] & bloomBitMask) != 0 {
			b.Blooms[i][byteIndex] |= bitMask
		}
	}

	b.NextBloomId++

	return nil
}

// Bitset returns the bit vector belonging to the given bit index after all
// Blooms have been added.
func (b *Generator) Bitset(idx uint) ([]byte, error) {
	if b.NextBloomId != uint16(params.BloomBitsSize) {
		return nil, errors.New("bloom not fully generated yet")
	}
	if idx >= types.BloomBitLength {
		return nil, errBloomBitOutOfBounds
	}
	return b.Blooms[idx][:], nil
}
