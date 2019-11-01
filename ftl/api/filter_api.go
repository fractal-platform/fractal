// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"context"
	"math/big"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/logbloom/bloomquery"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rpc"
	"github.com/fractal-platform/fractal/rpc/server"
	"github.com/fractal-platform/fractal/utils/log"
)

type FilterAPI struct {
	ftl fractal
}

func NewFilterAPI(ftl fractal) *FilterAPI {
	return &FilterAPI{ftl}
}

type FilterCriteria struct {
	BlockHash       *common.Hash     // used by ftl_getLogs, return logs only from block with this hash
	FromBlockHeight *hexutil.Big     // beginning of the queried range, nil means genesis block
	ToBlockHeight   *hexutil.Big     // end of the range, nil means latest block
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

func returnLogs(logs []*types.Log) []*types.Log {
	if logs == nil {
		return []*types.Log{}
	}
	return logs
}

func (api *FilterAPI) GetLogs(ctx context.Context, crit FilterCriteria) ([]*types.Log, error) {
	var filter *bloomquery.Filter
	if crit.BlockHash != nil {
		// Block filter requested, construct a single-shot filter
		filter = bloomquery.NewBlockFilter(api.ftl, *crit.BlockHash, crit.Addresses, crit.Topics)
	} else {
		// Convert the RPC block numbers into internal representations
		begin := int64(-1)
		if crit.FromBlockHeight != nil && (*big.Int)(crit.FromBlockHeight).Cmp(common.Big0) > 0 {
			begin = (*big.Int)(crit.FromBlockHeight).Int64()
		}
		end := int64(-1)
		if crit.ToBlockHeight != nil && (*big.Int)(crit.ToBlockHeight).Cmp(common.Big0) > 0 {
			end = (*big.Int)(crit.ToBlockHeight).Int64()
		}
		// Construct the range filter
		filter = bloomquery.NewRangeFilter(api.ftl, begin, end, crit.Addresses, crit.Topics)
	}
	// Run the filter and return all the logs
	logs, err := filter.Logs(ctx, params.ConfirmHeightDistance)
	if err != nil {
		return nil, err
	}
	return returnLogs(logs), err
}

func (api *FilterAPI) SubLogs(ctx context.Context, crit FilterCriteria, stableDistance uint64) (*rpcserver.Subscription, error) {
	notifier, supported := rpcserver.NotifierFromContext(ctx)
	if !supported {
		return &rpcserver.Subscription{}, rpc.ErrNotificationsUnsupported
	}

	rpcSub := notifier.CreateSubscription()
	var (
		unstableBlockLogMap = make(map[uint64][]*types.Log)

		ch  = make(chan types.BloomInsertEvent, 128)
		sub = api.ftl.SubscribeInsertBloomEvent(ch)
	)
	go func() {
		// a) history chain data
		log.Info("SubLogs: 1.Query logs from history data.")
		for {
			begin := int64(-1)
			if crit.FromBlockHeight != nil && (*big.Int)(crit.FromBlockHeight).Cmp(common.Big0) >= 0 {
				begin = (*big.Int)(crit.FromBlockHeight).Int64()
			}
			end := int64(-1)
			if crit.ToBlockHeight != nil && (*big.Int)(crit.ToBlockHeight).Cmp(common.Big0) >= 0 {
				end = (*big.Int)(crit.ToBlockHeight).Int64()
			}

			currentBlock := api.ftl.CurrentBlock(ctx)
			if currentBlock.Header.Height < stableDistance {
				break
			}
			stableHeight := currentBlock.Header.Height - stableDistance

			// Construct the range filter
			filter := bloomquery.NewRangeFilter(api.ftl, begin, end, crit.Addresses, crit.Topics)
			// Run the filter and return all the logs
			logs, err := filter.Logs(ctx, 0)
			if err != nil {
				log.Error("SubLogs: get history logs error", "err", err)
				break
			}
			for _, value := range logs {
				if value.BlockNumber > stableHeight {
					unstableBlockLogMap[value.BlockNumber] = append(unstableBlockLogMap[value.BlockNumber], value)
				} else {
					// is stable
					log.Info("SubLogs: log found in history data.", "log", value)
					if err := notifier.Notify(rpcSub.ID, []*types.Log{value}); err != nil {
						log.Error("SubLogs: notify history log error", "err", err)
					}
				}
			}
		}

		// b) new coming data
		log.Info("SubLogs: 2.Subscribe new logs.")
		for {
			select {
			case e := <-ch:
				// 1. check the coming block
				filter := bloomquery.NewBlockFilter(api.ftl, e.Block.FullHash(), crit.Addresses, crit.Topics)
				logs, err := filter.Logs(ctx, 0)
				if err != nil {
					log.Error("SubLogs: get coming block logs error", "err", err)
				} else {
					unstableBlockLogMap[e.Block.Header.Height] = logs // Directly overwritten, because the later data must be more accurate than before
				}

				// 2. notify the stable logs
				currentBlock := api.ftl.CurrentBlock(ctx)
				if currentBlock.Header.Height < stableDistance {
					continue
				}
				stableHeight := currentBlock.Header.Height - stableDistance
				for key, value := range unstableBlockLogMap {
					if len(value) > 0 && key <= stableHeight {
						// is stable
						log.Info("SubLogs: logs found in new coming data.", "stableHeight", stableHeight, "thisHeight", key, "logsNumber", len(value))
						if err := notifier.Notify(rpcSub.ID, value); err != nil {
							log.Error("SubLogs: notify new logs error", "err", err)
						}
						delete(unstableBlockLogMap, key)
					}
				}

			case <-rpcSub.Err():
				sub.Unsubscribe()
				return
			case <-notifier.Closed():
				sub.Unsubscribe()
				return
			}
		}
	}()

	return rpcSub, nil
}
