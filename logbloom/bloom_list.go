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
	s.BloomSaveBitFlags[byteIndex] |= byte(1) << (7 - remainderBit)
}

type BloomList struct {
	BListMap   map[uint64]*SectionBlooms
	HeadHeight uint64
	Mu         sync.RWMutex
}

var bloomList *BloomList
var fullBloomSaveBitFlags [params.BloomByteSize]byte
var once sync.Once

func GetBloomList() *BloomList {
	once.Do(func() {
		bloomList = &BloomList{
			BListMap: make(map[uint64]*SectionBlooms),
		}

		for i := range fullBloomSaveBitFlags {
			fullBloomSaveBitFlags[i] = 0xFF
		}
	})
	return bloomList
}

func (bloomList *BloomList) InsertBlockBloom(db dbaccessor.DatabaseReader, block *types.BlockHeader, cachedBloom *types.Bloom) uint64 {
	if block.Height > bloomList.HeadHeight {
		bloomList.HeadHeight = block.Height
	}
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

func (bloomList *BloomList) InsertBlockSectionBloom(db dbaccessor.DatabaseReader, blocks []*types.BlockHeader) uint64 {
	if blocks[len(blocks)-1].Height > bloomList.HeadHeight {
		bloomList.HeadHeight = blocks[len(blocks)-1].Height
	}
	currentSecId := blocks[0].Height / params.BloomBitsSize

	var sectionBlooms *SectionBlooms
	var exist bool
	if sectionBlooms, exist = bloomList.BListMap[currentSecId]; !exist {
		sectionBlooms = new(SectionBlooms)
		bloomList.BListMap[currentSecId] = sectionBlooms
	}

	// check
	if len(blocks) != int(params.BloomBitsSize) || blocks[0].Height%params.BloomBitsSize != 0 {
		log.Error("InsertBlockSectionBloom error: input is not a complete section")
		return currentSecId
	}

	for i := 0; i < int(params.BloomBitsSize); i++ {
		blockFullHash := blocks[i].FullHash()
		bloom, _ := dbaccessor.ReadBloom(db, blockFullHash)
		sectionBlooms.Blooms[i] = OneBloom{
			BlockFullHash: blockFullHash,
			BloomBit:      bloom,
		}
		sectionBlooms.markBitFlag(uint16(i))
	}

	return currentSecId
}

// The first one of the input blocks is the common ancestor, and its original descendants need to be reset.
func (bloomList *BloomList) ResetBlockBloomDb(db dbaccessor.DatabaseReader, blocks []*types.BlockHeader, headBlockCachedBloom *types.Bloom) ([]OneBloom, []*types.BlockHeader) {
	commonAncestor := blocks[0]
	commonAncestorHeight := commonAncestor.Height
	lastValidIndex := commonAncestorHeight % params.BloomBitsSize

	supplement := params.BloomBitsSize - lastValidIndex - 1

	if len(blocks)-1 < int(supplement) {
		log.Error("ResetBlockBloomDb: input blocks not enough", "input block num", len(blocks)-1)
		return nil, nil
	}

	supplementBloomArray := make([]OneBloom, supplement)
	for i := uint64(0); i < supplement; i++ {
		blockFullHash := blocks[i+1].FullHash()

		var bloom *types.Bloom
		if i+1 == uint64(len(blocks))-1 {
			// when process the currentHeader block (the last element of the input slice is the header block)
			bloom = headBlockCachedBloom
		}
		if bloom == nil {
			bloom, _ = dbaccessor.ReadBloom(db, blockFullHash)
		}
		supplementBloomArray[i] = OneBloom{
			BlockFullHash: blockFullHash,
			BloomBit:      bloom,
		}
	}
	leftBlocks := blocks[supplement+1:]
	return supplementBloomArray, leftBlocks
}

// The first one of the input blocks is the common ancestor, and its original descendants need to be reset.
func (bloomList *BloomList) ResetBlockBloomMap(db dbaccessor.DatabaseReader, blocks []*types.BlockHeader, headBlockCachedBloom *types.Bloom) []*types.BlockHeader {
	if blocks[len(blocks)-1].Height > bloomList.HeadHeight {
		bloomList.HeadHeight = blocks[len(blocks)-1].Height
	}
	commonAncestor := blocks[0]
	commonAncestorHeight := commonAncestor.Height
	resetSectionId := commonAncestorHeight / params.BloomBitsSize
	lastValidIndex := commonAncestorHeight % params.BloomBitsSize

	var sectionBlooms *SectionBlooms
	var exist bool
	if sectionBlooms, exist = bloomList.BListMap[resetSectionId]; !exist {
		sectionBlooms = new(SectionBlooms)
		bloomList.BListMap[resetSectionId] = sectionBlooms
	}

	blockIndex := 1
	resetId := lastValidIndex + 1

	for ; blockIndex < len(blocks) && resetId < params.BloomBitsSize; blockIndex, resetId = blockIndex+1, resetId+1 {
		blockFullHash := blocks[blockIndex].FullHash()

		var bloom *types.Bloom
		if blockIndex == len(blocks)-1 {
			// when process the currentHeader block (the last element of the input slice is the header block)
			bloom = headBlockCachedBloom
		}
		if bloom == nil {
			bloom, _ = dbaccessor.ReadBloom(db, blockFullHash)
		}

		sectionBlooms.Blooms[resetId] = OneBloom{
			BlockFullHash: blockFullHash,
			BloomBit:      bloom,
		}
		sectionBlooms.markBitFlag(uint16(resetId))
	}

	leftBlocks := blocks[blockIndex:]
	return leftBlocks
}

func (bloomList *BloomList) GetLast() uint64 {
	bloomList.Mu.RLock()
	defer bloomList.Mu.RUnlock()

	return bloomList.HeadHeight
}
