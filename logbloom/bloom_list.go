package logbloom

import (
	"sync"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

type OneBloom struct {
	BlockFullHash common.Hash
	BloomBit      *types.Bloom
}

type SectionBlooms struct {
	Blooms            [params.BloomBitsSize]OneBloom
	BloomSaveBitFlags [params.BloomByteSize]byte
}

func (s *SectionBlooms) IsFull() bool {
	return s.BloomSaveBitFlags == fullBloomSaveBitFlags
}

func (s *SectionBlooms) CheckBitFlag(bitIndex uint16) bool {
	byteIndex := bitIndex / 8
	remainderBit := bitIndex & 7
	return s.BloomSaveBitFlags[byteIndex]&(byte(1)<<(7-remainderBit)) > 0
}

func (s *SectionBlooms) markBitFlag(bitIndex uint16) {
	byteIndex := bitIndex / 8
	remainderBit := bitIndex & 7
	s.BloomSaveBitFlags[byteIndex] |= (byte(1) << (7 - remainderBit))
}

func (s *SectionBlooms) clearBitFlag(bitIndex uint16) {
	byteIndex := bitIndex / 8
	remainderBit := bitIndex & 7
	s.BloomSaveBitFlags[byteIndex] &= (^(byte(1) << (7 - remainderBit)))
}

type BloomList struct {
	BListMap map[uint64]*SectionBlooms
	Mu       sync.RWMutex
}

var bloomList *BloomList
var fullBloomSaveBitFlags [params.BloomByteSize]byte
var once sync.Once

func init() {
	// init fullBloomSaveBitFlags
	for i := range fullBloomSaveBitFlags {
		fullBloomSaveBitFlags[i] = 0xFF
	}
	log.Info("LogBloom: init fullBloomSaveBitFlags")
}

func GetBloomList() *BloomList {
	once.Do(func() {
		bloomList = &BloomList{
			BListMap: make(map[uint64]*SectionBlooms),
		}
	})
	return bloomList
}

func (bloomList *BloomList) InsertBlockBloom(db dbaccessor.DatabaseReader, block *types.BlockHeader, cachedBloom *types.Bloom) uint64 {
	currentSecId := block.Height / params.BloomBitsSize
	currentBloomId := block.Height % params.BloomBitsSize

	var sectionBlooms *SectionBlooms
	var exist bool
	if sectionBlooms, exist = bloomList.BListMap[currentSecId]; !exist {
		sectionBlooms = new(SectionBlooms)
		bloomList.BListMap[currentSecId] = sectionBlooms
	}

	blockFullHash := block.FullHash()

	var bloom = cachedBloom
	if cachedBloom == nil {
		bloom, _ = dbaccessor.ReadBloom(db, blockFullHash)
	}

	sectionBlooms.Blooms[currentBloomId] = OneBloom{
		BlockFullHash: blockFullHash,
		BloomBit:      bloom,
	}
	sectionBlooms.markBitFlag(uint16(currentBloomId))

	return currentSecId
}

func (bloomList *BloomList) DelBlockBloom(blockHeight uint64) {
	currentSecId := blockHeight / params.BloomBitsSize
	currentBloomId := blockHeight % params.BloomBitsSize

	sectionBlooms, exist := bloomList.BListMap[currentSecId]
	if exist {
		sectionBlooms.Blooms[currentBloomId] = OneBloom{}
		sectionBlooms.clearBitFlag(uint16(currentBloomId))
	}
}
