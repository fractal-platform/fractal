package config

import (
	"encoding/json"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/utils/log"
)

type CheckPoint struct {
	Hash   common.Hash `json:"hash" gencodec:"required"`
	Height uint64      `json:"height" gencodec:"required"`
	Round  uint64      `json:"round" gencodec:"required"`
}
type CheckPoints map[uint64]CheckPoint

func (cps *CheckPoints) UnmarshalJSON(data []byte) error {
	m := make(map[uint64]CheckPoint)
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	*cps = make(CheckPoints)
	for height, cp := range m {
		(*cps)[height] = cp
	}
	return nil
}

func GetCheckPointBelowBlock(block *types.Block, points *CheckPoints) CheckPoint {
	var checkPoint = CheckPoint{}
	//log.Info("GetCheckPointBelowBlock", "checkpoints", points, "blockHeight", block.Header.Height, "blockRound", block.Header.Round, "blockHash", block.FullHash())
	for height, point := range *points {
		if checkPoint == (CheckPoint{}) && height <= block.Header.Height {
			checkPoint = point
		} else if checkPoint != (CheckPoint{}) && height < block.Header.Height && point.Height >= checkPoint.Height {
			checkPoint = point
		}
	}
	return checkPoint
}

func GetLatestCheckPoint(points *CheckPoints) CheckPoint {
	if len(*points) <= 0 {
		log.Info("no checkpoints or checkpoints disabled, return nil")
	}
	var checkPoint = CheckPoint{}
	for _, point := range *points {
		if checkPoint == (CheckPoint{}) {
			checkPoint = point
		} else if point.Height >= checkPoint.Height {
			checkPoint = point
		}
	}
	return checkPoint
}
