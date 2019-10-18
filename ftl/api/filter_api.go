// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"math/big"

	"github.com/fractal-platform/fractal/common"
)

type FilterAPI struct {
	ftl fractal
}

func NewFilterAPI(ftl fractal) *FilterAPI {
	return &FilterAPI{ftl}
}

type FilterCriteria struct {
	BlockHash       *common.Hash     // used by ftl_getLogs, return logs only from block with this hash
	FromBlockHeight *big.Int         // beginning of the queried range, nil means genesis block
	ToBlockHeight   *big.Int         // end of the range, nil means latest block
	Addresses       []common.Address // restricts matches to events created by specific contracts

	// The Topic list restricts matches to particular event topics. Each event has a list
	// of topics. Topics matches a prefix of that list. An empty element slice matches any
	// topic. Non-empty elements represent an alternative that matches any of the
	// contained topics.
	//
	// Examples:
	// {} or nil          matches any topic list
	// {{A}}              matches topic A in first position
	// {{}, {B}}          matches any topic in first position, B in second position
	// {{A}, {B}}         matches topic A in first position, B in second position
	// {{A, B}}, {C, D}}  matches topic (A OR B) in first position, (C OR D) in second position
	Topics [][]common.Hash
}

//func returnLogs(logs []*types.Log) []*types.Log {
//	if logs == nil {
//		return []*types.Log{}
//	}
//	return logs
//}

// TODO: Temporarily not open bloom
//func (api *FilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
//	var filter *bloomquery.Filter
//	if crit.BlockHash != nil {
//		// Block filter requested, construct a single-shot filter
//		filter = bloomquery.NewBlockFilter(api.ftl, *crit.BlockHash, crit.Addresses, crit.Topics)
//	} else {
//		// Convert the RPC block numbers into internal representations
//		begin := int64(-1)
//		if crit.FromBlockHeight != nil && crit.FromBlockHeight.Cmp(common.Big0) > 0 {
//			begin = crit.FromBlockHeight.Int64()
//		}
//		end := int64(-1)
//		if crit.ToBlockHeight != nil && crit.ToBlockHeight.Cmp(common.Big0) > 0 {
//			end = crit.ToBlockHeight.Int64()
//		}
//		// Construct the range filter
//		filter = bloomquery.NewRangeFilter(api.ftl, begin, end, crit.Addresses, crit.Topics)
//	}
//	// Run the filter and return all the logs
//	logs, err := filter.Logs(ctx)
//	if err != nil {
//		return nil, err
//	}
//	return returnLogs(logs), err
//}
