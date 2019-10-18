package bloomstorage

import (
	"errors"

	"github.com/fractal-platform/fractal/common/bitutil"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/logbloom"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

type SectionWriter struct {
	chainDb    dbwrapper.Database // Chain database to index the data from
	gen        *Generator         // generator to rotate the bloom bits crating the bloom index
	accSection uint64             // Section is the section number being processed currently
}

func NewSectionWriter(chainDb dbwrapper.Database) *SectionWriter {
	return &SectionWriter{
		chainDb: chainDb,
	}
}

func (s *SectionWriter) WriteSection(secNum uint64, bloomSlice []logbloom.OneBloom) error {
	bloomNum := len(bloomSlice)
	log.Info("Processing new chain section", "section", secNum, "bloomNum", bloomNum)

	// Reset and partial processing
	if bloomNum == 0 {
		return errors.New("bloom section size is 0")
	}

	s.reset(secNum, uint16(bloomNum))
	start := int(params.BloomBitsSize) - bloomNum

	for i := range bloomSlice {
		if err := s.process(uint16(start+i), bloomSlice[i].BloomBit); err != nil {
			log.Error("Pack Bloom Process error", "error", err)
			return err
		}
	}

	if err := s.commit(); err != nil {
		return err
	}
	return nil
}

func (s *SectionWriter) ClearSection(secNum uint64) {
	batch := s.chainDb.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		dbaccessor.DeleteBloomBits(batch, uint(i), secNum)
	}

	dbaccessor.WriteBloomSectionSavedFlag(batch, secNum, false)
	batch.Write()
}

func (s *SectionWriter) ReplaceSectionBit(section uint64, bloomHeight uint64, bloomBit *types.Bloom) error {
	gen := NewGenerator()

	replaceBitIndex := uint16(bloomHeight % params.BloomBitsSize)
	replaceByteIndex := replaceBitIndex / 8
	bitMask := byte(1) << byte(7-replaceBitIndex&7)
	zeroBitMask := byte(0xFF) - bitMask

	// read
	for i := 0; i < types.BloomBitLength; i++ {
		var compVector, blob []byte
		var err error
		if compVector, err = dbaccessor.ReadBloomBits(s.chainDb, uint(i), section); err != nil {
			return err
		}
		if blob, err = bitutil.DecompressBytes(compVector, int(params.BloomByteSize)); err != nil {
			return err
		}
		copy(gen.Blooms[i][:], blob)
		dbaccessor.DeleteBloomBits(s.chainDb, uint(i), section)
	}

	// replace
	for i := 0; i < types.BloomBitLength; i++ {
		bloomByteIndex := types.BloomByteLength - 1 - i/8
		bloomBitMask := byte(1) << byte(i&7)

		if (bloomBit[bloomByteIndex] & bloomBitMask) == 0 {
			gen.Blooms[i][replaceByteIndex] &= zeroBitMask
		} else {
			gen.Blooms[i][replaceByteIndex] |= bitMask
		}
	}

	gen.NextBloomId = uint16(params.BloomBitsSize)
	s.gen, s.accSection = gen, section
	err := s.commit()

	return err
}

// Reset starting a new bloombits index section.
func (s *SectionWriter) reset(section uint64, bloomNum uint16) error {
	gen := NewGenerator()
	if bloomNum > 0 && bloomNum < uint16(params.BloomBitsSize) {
		if !dbaccessor.ReadBloomSectionSavedFlag(s.chainDb, section) {
			log.Error("BloomIndexer Reset error, cannot find section in database", "section", section)
			return ErrCannotFindSection
		}

		validBitIndex := uint16(params.BloomBitsSize) - bloomNum - 1
		validByteIndex := validBitIndex / 8
		remainderBit := validBitIndex % 8

		for i := 0; i < types.BloomBitLength; i++ {
			var compVector, blob []byte
			var err error
			if compVector, err = dbaccessor.ReadBloomBits(s.chainDb, uint(i), section); err != nil {
				return err
			}
			if blob, err = bitutil.DecompressBytes(compVector, int(params.BloomByteSize)); err != nil {
				return err
			}
			copy(gen.Blooms[i][:validByteIndex], blob[:validByteIndex])
			gen.Blooms[i][validByteIndex] = (byte(0xFF) << (7 - remainderBit)) & blob[validByteIndex]
			dbaccessor.DeleteBloomBits(s.chainDb, uint(i), section)
		}

		dbaccessor.WriteBloomSectionSavedFlag(s.chainDb, section, false)
		gen.NextBloomId = validBitIndex + 1
	}
	s.gen, s.accSection = gen, section
	return nil
}

// Process adding a new header's bloom into the index.
func (s *SectionWriter) process(index uint16, bloom *types.Bloom) error {
	err := s.gen.AddBloom(index, bloom)
	return err
}

// Commit finalizing the bloom section and writing it out into the database.
func (s *SectionWriter) commit() error {
	batch := s.chainDb.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		bits, err := s.gen.Bitset(uint(i))
		if err != nil {
			return err
		}
		dbaccessor.WriteBloomBits(batch, uint(i), s.accSection, bitutil.CompressBytes(bits))
	}

	dbaccessor.WriteBloomSectionSavedFlag(batch, s.accSection, true)

	return batch.Write()
}
