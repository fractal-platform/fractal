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
	if bloomNum != int(params.BloomBitsSize) {
		return errors.New("bloom section size is not complete")
	}

	s.reset(secNum)

	for i := range bloomSlice {
		if err := s.process(uint16(i), bloomSlice[i].BloomBit); err != nil {
			log.Error("Pack Bloom Process error", "error", err)
			return err
		}
	}

	if err := s.commit(); err != nil {
		return err
	}
	return nil
}

func (s *SectionWriter) ReplaceSectionBit(section uint64, bloomHeight uint64, bloomBit *types.Bloom) error {
	s.reset(section)

	replaceBitIndex := uint16(bloomHeight % params.BloomBitsSize)
	replaceByteIndex := replaceBitIndex / 8
	bitMask := byte(1) << byte(7-replaceBitIndex&7)
	zeroBitMask := ^bitMask

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
		copy(s.gen.Blooms[i][:], blob)
	}

	// check if need to replace
	var noChange = true
	for i := 0; i < types.BloomBitLength; i++ {
		bloomByteIndex := types.BloomByteLength - 1 - i/8
		bloomBitMask := byte(1) << byte(i&7)

		if (bloomBit[bloomByteIndex] & bloomBitMask) == 0 {
			if s.gen.Blooms[i][replaceByteIndex]&bitMask != 0 {
				noChange = false
				s.gen.Blooms[i][replaceByteIndex] &= zeroBitMask
			}
		} else {
			if s.gen.Blooms[i][replaceByteIndex]&bitMask == 0 {
				noChange = false
				s.gen.Blooms[i][replaceByteIndex] |= bitMask
			}
		}
	}

	if noChange {
		log.Info("ReplaceSectionBit: section has no change, don't need to commit.", "section", section, "bloomHeight", bloomHeight)
		s.gen = nil
		return nil
	}

	// clear
	dbaccessor.WriteBloomSectionSavedFlag(s.chainDb, section, false)

	// recommit
	s.gen.NextBloomId = uint16(params.BloomBitsSize)
	err := s.commit()

	return err
}

func (s *SectionWriter) ClearSection(secNum uint64) {
	batch := s.chainDb.NewBatch()
	for i := 0; i < types.BloomBitLength; i++ {
		dbaccessor.DeleteBloomBits(batch, uint(i), secNum)
	}

	dbaccessor.WriteBloomSectionSavedFlag(batch, secNum, false)
	batch.Write()
}

// Reset starting a new bloombits index section.
func (s *SectionWriter) reset(section uint64) {
	gen := NewGenerator()
	s.gen, s.accSection = gen, section
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

	s.gen = nil

	return batch.Write()
}
