package bloomquery

import (
	"context"
	"errors"
	"math/big"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/logbloom"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/utils/log"
)

const (
	// bloomFilterThreads is the number of goroutines used locally per filter to
	// multiplex requests onto the global servicing goroutines.
	bloomFilterThreads = 3

	// bloomRetrievalBatch is the maximum number of bloom bit retrievals to service
	// in a single batch.
	bloomRetrievalBatch = 16

	// bloomRetrievalWait is the maximum time to wait for enough bloom bit requests
	// to accumulate request an entire batch (avoiding hysteresis).
	bloomRetrievalWait = time.Duration(0)
)

var (
	errGetIndexedLog           = errors.New("get indexed log error")
	errGetUnIndexedLog         = errors.New("get unIndexed log error")
	errGetIndexAndUnIndexedLog = errors.New("get indexed log and unIndexed log error")
)

type Backend interface {
	ChainDb() dbwrapper.Database
	GetBlock(ctx context.Context, fullHash common.Hash) *types.Block
	CurrentBlock(ctx context.Context) *types.Block
	GetMainBranchBlock(height uint64) (*types.Block, error)
	GetLogs(ctx context.Context, blockHash common.Hash) [][]*types.Log
	BloomRequestsReceiver() chan chan *Retrieval
}

// Filter can be used to retrieve and filter logs.
type Filter struct {
	backend Backend

	db        dbwrapper.Database
	addresses []common.Address
	topics    [][]common.Hash

	block      common.Hash // Block hash if filtering a single block
	begin, end int64       // Range interval if filtering multiple blocks

	matcher *Matcher
}

// NewRangeFilter creates a new filter which uses a bloom filter on blocks to
// figure out whether a particular block is interesting or not.
func NewRangeFilter(backend Backend, begin, end int64, addresses []common.Address, topics [][]common.Hash) *Filter {
	// Flatten the address and topic filter clauses into a single bloombits filter
	// system. Since the bloombits are not positional, nil topics are permitted,
	// which get flattened into a nil byte slice.
	var filters [][][]byte
	if len(addresses) > 0 {
		filter := make([][]byte, len(addresses))
		for i, address := range addresses {
			filter[i] = address.Bytes()
		}
		filters = append(filters, filter)
	}
	for _, topicList := range topics {
		filter := make([][]byte, len(topicList))
		for i, topic := range topicList {
			filter[i] = topic.Bytes()
		}
		filters = append(filters, filter)
	}

	// Create a generic filter and convert it into a range filter
	filter := newFilter(backend, addresses, topics)

	filter.matcher = NewMatcher(filters)
	filter.begin = begin
	filter.end = end

	return filter
}

// NewBlockFilter creates a new filter which directly inspects the contents of
// a block to figure out whether it is interesting or not.
func NewBlockFilter(backend Backend, block common.Hash, addresses []common.Address, topics [][]common.Hash) *Filter {
	// Create a generic filter and convert it into a block filter
	filter := newFilter(backend, addresses, topics)
	filter.block = block
	return filter
}

// newFilter creates a generic filter that can either filter based on a block hash,
// or based on range queries. The search criteria needs to be explicitly set.
func newFilter(backend Backend, addresses []common.Address, topics [][]common.Hash) *Filter {
	return &Filter{
		backend:   backend,
		addresses: addresses,
		topics:    topics,
		db:        backend.ChainDb(),
	}
}

// Logs searches the blockchain for matching log entries, returning all from the
// first block that contains matches, updating the start of the filter accordingly.
func (f *Filter) Logs(ctx context.Context, stableDistance uint64) ([]*types.Log, error) {
	// If we're doing singleton block filtering, execute and return
	if f.block != (common.Hash{}) {
		block := f.backend.GetBlock(ctx, f.block)
		if block == nil {
			return nil, errors.New("unknown block")
		}
		return f.blockLogs(ctx, block)
	}

	begin := uint64(f.begin)
	if f.begin == -1 {
		begin = 0
	}

	end := uint64(f.end)
	if f.end == -1 {
		end = f.backend.CurrentBlock(ctx).Header.Height
	}

	if begin == 0 && end == 0 {
		return nil, nil
	}

	// Gather all indexed logs, and finish with non indexed ones
	var (
		logs                                []*types.Log
		err, indexedLogErr, unIndexedLogErr error
	)

	var sectionIdInDb []uint64
	var blockHeightNotInDb []uint64

	for i := begin; i <= end; {
		sectionId := i / params.BloomBitsSize
		if dbaccessor.ReadBloomSectionSavedFlag(f.db, sectionId) {
			sectionIdInDb = append(sectionIdInDb, sectionId)
			nextSectionStart := (sectionId + 1) * params.BloomBitsSize
			i = nextSectionStart
		} else {
			blockHeightNotInDb = append(blockHeightNotInDb, i)
			i++
		}
	}

	logs, indexedLogErr = f.indexedLogs(ctx, sectionIdInDb, begin, end)
	log.Info("Logs: indexedLogs", "len", len(logs), "err", indexedLogErr)
	rest, unIndexedLogErr := f.unIndexedLogs(ctx, blockHeightNotInDb, stableDistance)
	log.Info("Logs: unIndexedLogs", "len", len(rest), "err", unIndexedLogErr)
	logs = append(logs, rest...)

	if indexedLogErr == nil {
		if unIndexedLogErr == nil {
			err = nil
		} else {
			err = errGetUnIndexedLog
		}
	} else {
		if unIndexedLogErr == nil {
			err = errGetIndexedLog
		} else {
			err = errGetIndexAndUnIndexedLog
		}
	}
	return logs, err
}

// indexedLogs returns the logs matching the filter criteria based on the bloom
// bits indexed available locally or via the network.
func (f *Filter) indexedLogs(ctx context.Context, sectionIds []uint64, begin, end uint64) ([]*types.Log, error) {
	// Create a matcher session and request servicing from the backend
	matches := make(chan uint64, 64)

	session, err := f.matcher.Start(ctx, f.db, sectionIds, begin, end, matches)
	if err != nil {
		return nil, err
	}
	defer session.Close()

	session.MultiGetQueryTaskLoop(ctx, f.backend.BloomRequestsReceiver())

	// Iterate over the matches until exhausted or context closed
	var logs []*types.Log

	for {
		select {
		case height, ok := <-matches:
			// Abort if all matches have been fulfilled
			if !ok {
				err := session.Error()
				return logs, err
			}

			block, err := f.backend.GetMainBranchBlock(height)
			if err != nil {
				return logs, err
			}

			if block == nil || block.Header.Height != height {
				return logs, errors.New("indexedLogs: find wrong block")
			}

			found := f.checkMatches(ctx, block.FullHash())
			logs = append(logs, found...)

		case <-ctx.Done():
			return logs, ctx.Err()
		}
	}
}

// unIndexedLogs returns the logs matching the filter criteria based on raw block
// iteration and bloom matching.
func (f *Filter) unIndexedLogs(ctx context.Context, blockHeights []uint64, stableDistance uint64) ([]*types.Log, error) {
	var logs []*types.Log

	bloomInfoList := logbloom.GetBloomList()
	headHeight := f.backend.CurrentBlock(ctx).Header.Height
	if headHeight < stableDistance {
		return logs, nil
	}

	stableHeight := headHeight - stableDistance

	for _, height := range blockHeights {
		if height > stableHeight {
			return logs, nil
		}

		sectionId := height / params.BloomBitsSize
		bloomId := height % params.BloomBitsSize

		bloomInfoList.Mu.RLock()
		if sectionBloom, exist := bloomInfoList.BListMap[sectionId]; exist {
			if sectionBloom.CheckBitFlag(uint16(bloomId)) {
				oneBloom := sectionBloom.Blooms[uint16(bloomId)]
				if oneBloom.BlockFullHash != (common.Hash{}) {
					if block := f.backend.GetBlock(ctx, oneBloom.BlockFullHash); block != nil {
						found, _ := f.blockLogs(ctx, block)
						if found != nil {
							logs = append(logs, found...)
						}
					}
				}
			}
		}
		bloomInfoList.Mu.RUnlock()
	}

	return logs, nil
}

// blockLogs returns the logs matching the filter criteria within a single block.
func (f *Filter) blockLogs(ctx context.Context, block *types.Block) ([]*types.Log, error) {
	blockBloom, err := dbaccessor.ReadBloom(f.db, block.FullHash())
	if err != nil {
		return nil, err
	}
	var logs []*types.Log
	if bloomFilter(blockBloom, f.addresses, f.topics) {
		found := f.checkMatches(ctx, block.FullHash())
		if found != nil {
			logs = append(logs, found...)
		}
	}
	return logs, nil
}

// checkMatches checks if the logs belonging to the given block contain any log events that
// match the filter criteria. This function is called when the bloom filter signals a potential match.
func (f *Filter) checkMatches(ctx context.Context, blockHash common.Hash) (logs []*types.Log) {
	// Get the logs of the block
	logsList := f.backend.GetLogs(ctx, blockHash)
	if logsList == nil {
		return nil
	}
	var unfiltered []*types.Log
	for _, logs := range logsList {
		unfiltered = append(unfiltered, logs...)
	}
	logs = filterLogs(unfiltered, nil, nil, f.addresses, f.topics)

	return logs
}

func includes(addresses []common.Address, a common.Address) bool {
	for _, addr := range addresses {
		if addr == a {
			return true
		}
	}

	return false
}

// filterLogs creates a slice of logs matching the given criteria.
func filterLogs(logs []*types.Log, fromBlock, toBlock *big.Int, addresses []common.Address, topics [][]common.Hash) []*types.Log {
	var ret []*types.Log
Logs:
	for _, log := range logs {
		if fromBlock != nil && fromBlock.Int64() >= 0 && fromBlock.Uint64() > log.BlockNumber {
			continue
		}
		if toBlock != nil && toBlock.Int64() >= 0 && toBlock.Uint64() < log.BlockNumber {
			continue
		}

		if len(addresses) > 0 && !includes(addresses, log.Address) {
			continue
		}
		// If the to filtered topics is greater than the amount of topics in logs, skip.
		if len(topics) > len(log.Topics) {
			continue Logs
		}
		for i, sub := range topics {
			match := len(sub) == 0 // empty rule set == wildcard
			for _, topic := range sub {
				if log.Topics[i] == topic {
					match = true
					break
				}
			}
			if !match {
				continue Logs
			}
		}
		ret = append(ret, log)
	}
	return ret
}

func bloomFilter(bloom *types.Bloom, addresses []common.Address, topics [][]common.Hash) bool {
	if len(addresses) > 0 {
		var included bool
		for _, addr := range addresses {
			if types.BloomLookup(bloom, addr) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	for _, sub := range topics {
		included := len(sub) == 0 // empty rule set == wildcard
		for _, topic := range sub {
			if types.BloomLookup(bloom, topic) {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}
	return true
}
