// Copyright 2018 The go-fractal Authors
// This file is part of the go-fractal library.

// Fractal implements the Fractal full node service.
package api

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/common/hexutil"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/core/wasm"
	"github.com/fractal-platform/fractal/params"
	"github.com/fractal-platform/fractal/rlp"
	"github.com/fractal-platform/fractal/rpc/client"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils/log"
)

var (
	batchSendThreadNumber = 100

	ErrBatchPackNotEnabled = errors.New("batch pack not enabled")
)

// RPCTransaction represents a transaction that will serialize to the RPC representation of a transaction
type RPCTransaction struct {
	From      common.Address  `json:"from"`
	Hash      common.Hash     `json:"hash"`
	Nonce     hexutil.Uint64  `json:"nonce"`
	To        *common.Address `json:"to"`
	Value     *hexutil.Big    `json:"value"`
	Price     *hexutil.Big    `json:"gasPrice"`
	GasLimit  hexutil.Uint64  `json:"gas"`
	Payload   hexutil.Bytes   `json:"input"`
	Broadcast bool            `json:"broadcast"`
	V         *hexutil.Big    `json:"v"`
	R         *hexutil.Big    `json:"r"`
	S         *hexutil.Big    `json:"s"`

	BlockHash common.Hash    `json:"blockHash"`
	Receipt   *types.Receipt `json:"receipt"`
}

// newRPCTransaction returns a transaction that will serialize to the RPC
// representation, with the given location metadata set (if available).
func newRPCTransaction(tx *types.Transaction, blockHash common.Hash, receipt *types.Receipt, chainConfig *config.ChainConfig) *RPCTransaction {
	var signer = types.MakeSigner(chainConfig.TxSignerType, chainConfig.ChainID)
	from, _ := types.Sender(signer, tx)
	v, r, s := tx.RawSignatureValues()

	result := &RPCTransaction{
		From:      from,
		Hash:      tx.Hash(),
		Nonce:     hexutil.Uint64(tx.Nonce()),
		To:        tx.To(),
		Value:     (*hexutil.Big)(tx.Value()),
		Price:     (*hexutil.Big)(tx.GasPrice()),
		GasLimit:  hexutil.Uint64(tx.Gas()),
		Payload:   hexutil.Bytes(tx.Data()),
		Broadcast: tx.Broadcast(),
		V:         (*hexutil.Big)(v),
		R:         (*hexutil.Big)(r),
		S:         (*hexutil.Big)(s),

		BlockHash: blockHash,
		Receipt:   receipt,
	}
	return result
}

// newRPCPendingTransaction returns a pending transaction that will serialize to the RPC representation
func newRPCPendingTransaction(tx *types.Transaction, config *config.ChainConfig) *RPCTransaction {
	return newRPCTransaction(tx, common.Hash{}, nil, config)
}

type batchTxTasks struct {
	packerId uint32
	txs      types.Transactions
	errs     []string
}

// TxPoolAPI offers and API for the transaction pool. It only operates on data that is non confidential.
type TxPoolAPI struct {
	ftl         fractal
	chainConfig *config.ChainConfig

	batchSendEnable   bool
	batchSendChan     chan chan *batchTxTasks
	batchSendCacheMap map[uint32]types.Transactions // packer id -> transaction list
	cacheMapMu        sync.Mutex
}

// NewTxPoolAPI creates a new tx pool service that gives information about the transaction pool.
func NewTxPoolAPI(ftl fractal, config *config.Config) *TxPoolAPI {
	txPoolApi := &TxPoolAPI{
		ftl:         ftl,
		chainConfig: config.ChainConfig,
	}

	// not enable batch pack
	if config.TxBatchSendToPackInterval <= 0 {
		return txPoolApi
	}

	// enable batch pack
	txPoolApi.batchSendEnable = true
	txPoolApi.batchSendChan = make(chan chan *batchTxTasks, 3)
	txPoolApi.batchSendCacheMap = make(map[uint32]types.Transactions)
	ticker := time.NewTicker(time.Duration(config.TxBatchSendToPackInterval) * time.Millisecond)
	go txPoolApi.makeBatchPeriodically(ticker)
	txPoolApi.processBatchSend(batchSendThreadNumber)

	return txPoolApi
}

func (s *TxPoolAPI) makeBatchPeriodically(ticker *time.Ticker) {
	for {
		<-ticker.C
		s.cacheMapMu.Lock()
		for packerId, transactions := range s.batchSendCacheMap {
			go func(packerId uint32, transactions types.Transactions) {
				task := make(chan *batchTxTasks)

				s.batchSendChan <- task
				task <- &batchTxTasks{packerId: packerId, txs: transactions}

				result := <-task
				for i := 0; i < len(result.errs); i++ {
					if result.errs[i] != "" {
						log.Error("transaction send error", "packerId", packerId, "txHash", transactions[i].Hash(), "txNonce", transactions[i].Nonce(), "err", result.errs[i])
					}
				}
			}(packerId, transactions)

			delete(s.batchSendCacheMap, packerId)
		}
		s.cacheMapMu.Unlock()
	}
}

func (s *TxPoolAPI) processBatchSend(threadNum int) {
	for i := 0; i < threadNum; i++ {
		go func() {
			for {
				request := <-s.batchSendChan
				task := <-request

				currentBlock := s.ftl.BlockChain().CurrentBlock()
				packerInfo, _, err := s.ftl.BlockChain().GetPrePackerInfoByIndex(currentBlock, task.packerId)
				if err != nil {
					log.Error("cannot get packerInfo", "packerIndex", task.packerId, "err", err)
					request <- task
					continue
				}
				rpcAddr := packerInfo.RpcAddress
				client, err := rpcclient.Dial(rpcAddr)
				if err != nil {
					log.Error("connect to rpc error", "rpc", rpcAddr, "packerIndex", task.packerId, "err", err)
					request <- task
					continue
				}
				encodedTxs, err := rlp.EncodeToBytes(task.txs)
				if err != nil {
					log.Error("encode txs error", "err", err)
					request <- task
					continue
				}

				var packErrors []string
				err = client.Call(&packErrors, "pack_sendRawTransactions", encodedTxs)
				if err != nil {
					log.Error("send tx to packer error:", "rpc", rpcAddr, "packerIndex", task.packerId, "err", err)
				}
				log.Info("send one batch to packer", "packerId", task.packerId, "tx num", len(task.txs))
				task.errs = packErrors
				request <- task
			}
		}()
	}
}

// ReadTransaction retrieves a specific transaction from the database, along with
// its added positional metadata.
func ReadTransaction(bc *chain.BlockChain, hash common.Hash) (*types.Transaction, common.Hash) {
	entry, err := dbaccessor.ReadTxLookupEntry(bc.Database(), hash)

	if err != nil {
		log.Error("ReadTransaction: cannot find transaction in db", "hash", hash, "err", err)
		return nil, common.Hash{}
	}

	blockFullHash := entry.BlockFullHash
	block := bc.GetBlock(blockFullHash)
	if !bc.IsInMainBranch(block) {
		log.Error("ReadTransaction: transaction is not in main chain")
		return nil, common.Hash{}
	}

	txPackageIndex := entry.TxPackageIndex
	txIndex := entry.TxIndex

	if txPackageIndex == types.NotInPackage {
		// tx not in a package
		if len(block.Body.Transactions) <= int(txIndex) {
			log.Error("Transaction referenced missing(block)", "hash", blockFullHash, "index", txIndex)
			return nil, common.Hash{}
		}

		return block.Body.Transactions[txIndex], blockFullHash
	} else {
		// tx in a package
		txPackage := bc.GetTxPackage(block.Body.TxPackageHashes[txPackageIndex])
		if txPackage == nil {
			log.Error("TxPackage referenced missing(block)", "hash", block.Body.TxPackageHashes[txPackageIndex], "txPackageIndex", txIndex)
			return nil, common.Hash{}
		}

		if len(txPackage.Transactions()) <= int(txIndex) {
			log.Error("Transaction referenced missing(txPackage)", "index", txIndex)
			return nil, common.Hash{}
		}

		return txPackage.Transactions()[txIndex], blockFullHash
	}
}

// GetTransactionByHash returns the transaction for the given hash
func (s *TxPoolAPI) GetTransactionByHash(ctx context.Context, hash common.Hash) *RPCTransaction {
	// Try to return an already finalized transaction
	tx, blockHash := ReadTransaction(s.ftl.BlockChain(), hash);
	if tx == nil {
		tx, blockHash = s.ftl.BlockChain().SearchTransactionInCache(hash)
	}
	if tx != nil {
		var txReceipt *types.Receipt
		receipts := dbaccessor.ReadReceipts(s.ftl.ChainDb(), blockHash)
		for _, value := range receipts {
			if value.TxHash == hash {
				txReceipt = value
				break
			}
		}
		return newRPCTransaction(tx, blockHash, txReceipt, s.chainConfig)
	}
	// No finalized transaction, try to retrieve it from the pool
	if tx := s.ftl.TxPool().Get(hash); tx != nil {
		return newRPCPendingTransaction(tx.(*types.Transaction), s.chainConfig)
	}
	// Transaction unknown, return as such
	return nil
}

// Content returns the transactions contained within the transaction pool.
func (s *TxPoolAPI) Content() map[string]map[string]map[string]*RPCTransaction {
	content := map[string]map[string]map[string]*RPCTransaction{
		"queued": make(map[string]map[string]*RPCTransaction),
	}
	queue := s.ftl.TxPool().Content()

	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]*RPCTransaction)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = newRPCPendingTransaction(tx.(*types.Transaction), s.chainConfig)
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

// Status returns the number of pending and queued transaction in the pool.
func (s *TxPoolAPI) Status() map[string]hexutil.Uint {
	queue := s.ftl.TxPool().Stats()
	return map[string]hexutil.Uint{
		"queued": hexutil.Uint(queue),
	}
}

// Inspect retrieves the content of the transaction pool and flattens it into an
// easily inspectable list.
func (s *TxPoolAPI) Inspect() map[string]map[string]map[string]string {
	content := map[string]map[string]map[string]string{
		"queued": make(map[string]map[string]string),
	}
	queue := s.ftl.TxPool().Content()

	// Define a formatter to flatten a transaction into a string
	var format = func(tx *types.Transaction) string {
		if to := tx.To(); to != nil {
			return fmt.Sprintf("%s: %v nFra", tx.To().Hex(), tx.Value())
		}
		return fmt.Sprintf("contract creation: %v nFra", tx.Value())
	}
	// Flatten the queued transactions
	for account, txs := range queue {
		dump := make(map[string]string)
		for _, tx := range txs {
			dump[fmt.Sprintf("%d", tx.Nonce())] = format(tx.(*types.Transaction))
		}
		content["queued"][account.Hex()] = dump
	}
	return content
}

func (s *TxPoolAPI) GetTransactionNonce(ctx context.Context, address common.Address) *hexutil.Uint64 {
	nonce := s.ftl.TxPool().GetNonce(address, nil)
	return (*hexutil.Uint64)(&nonce)
}

// GetBlockTransactionCountByHash returns the number of transactions in the block with the given hash.
func (s *TxPoolAPI) GetBlockTransactionCountByHash(ctx context.Context, blockHash common.Hash) *hexutil.Uint {
	if block := s.ftl.GetBlock(ctx, blockHash); block != nil {
		n := hexutil.Uint(len(block.Body.Transactions))
		return &n
	}
	return nil
}

func (s *TxPoolAPI) SendRawTransaction(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}

	if tx.Broadcast() {
		if err := s.ftl.TxPool().AddLocal(tx); err != nil {
			return common.Hash{}, err
		}
		return tx.Hash(), nil
	} else {
		currentBlock := s.ftl.BlockChain().CurrentBlock()
		packerNumber, err := s.ftl.BlockChain().GetPrePackerNumber(currentBlock)
		if err != nil {
			return common.Hash{}, err
		}

		txPackingHashUint64 := tx.PackingHashUint64(s.ftl.Signer())

		var (
			packerInfo *types.PackerInfo
			client     *rpcclient.Client
		)

		packerIndex := uint32(txPackingHashUint64 % s.chainConfig.PackerGroupSize)
		var allowedPackerIndexList []uint32
		for packerIndex < packerNumber {
			allowedPackerIndexList = append(allowedPackerIndexList, packerIndex)
			packerIndex += uint32(s.chainConfig.PackerGroupSize)
		}

		for {
			if len(allowedPackerIndexList) == 0 {
				break
			}

			r := rand.New(rand.NewSource(time.Now().UnixNano()))
			groupId := r.Intn(len(allowedPackerIndexList))
			randPackerIndex := allowedPackerIndexList[groupId]

			packerInfo, _, err = s.ftl.BlockChain().GetPrePackerInfoByIndex(currentBlock, randPackerIndex)
			if err != nil {
				log.Error("cannot get packerInfo", "packerIndex", randPackerIndex, "err", err)
				allowedPackerIndexList = append(allowedPackerIndexList[0:groupId], allowedPackerIndexList[groupId+1:]...)
				continue
			}
			rpcAddr := packerInfo.RpcAddress

			client, err = rpcclient.Dial(rpcAddr)
			if err != nil {
				log.Error("connect to rpc error", "rpc", rpcAddr, "packerIndex", randPackerIndex, "err", err)
				allowedPackerIndexList = append(allowedPackerIndexList[0:groupId], allowedPackerIndexList[groupId+1:]...)
				continue
			}
			var hash common.Hash
			err = client.Call(&hash, "pack_sendRawTransaction", encodedTx)
			if err != nil {
				log.Error("send tx to packer error:", "rpc", rpcAddr, "packerIndex", randPackerIndex, "err", err)
				allowedPackerIndexList = append(allowedPackerIndexList[0:groupId], allowedPackerIndexList[groupId+1:]...)
				continue
			}
			return hash, nil
		}

		return common.Hash{}, err
	}
}

func (s *TxPoolAPI) SendRawTransactionPeriodically(ctx context.Context, encodedTx hexutil.Bytes) (common.Hash, error) {
	if !s.batchSendEnable {
		return common.Hash{}, ErrBatchPackNotEnabled
	}

	tx := new(types.Transaction)
	if err := rlp.DecodeBytes(encodedTx, tx); err != nil {
		return common.Hash{}, err
	}

	if tx.Broadcast() {
		if err := s.ftl.TxPool().AddLocal(tx); err != nil {
			return common.Hash{}, err
		}
		return tx.Hash(), nil
	} else {
		currentBlock := s.ftl.BlockChain().CurrentBlock()
		packerNumber, err := s.ftl.BlockChain().GetPrePackerNumber(currentBlock)
		if err != nil {
			return common.Hash{}, err
		}

		txPackingHashUint64 := tx.PackingHashUint64(s.ftl.Signer())

		packerIndex := uint32(txPackingHashUint64 % s.chainConfig.PackerGroupSize)
		var allowedPackerIndexList []uint32
		for packerIndex < packerNumber {
			allowedPackerIndexList = append(allowedPackerIndexList, packerIndex)
			packerIndex += uint32(s.chainConfig.PackerGroupSize)
		}
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		groupId := r.Intn(len(allowedPackerIndexList))
		randPackerIndex := allowedPackerIndexList[groupId]

		s.cacheMapMu.Lock()
		s.batchSendCacheMap[randPackerIndex] = append(s.batchSendCacheMap[randPackerIndex], tx)
		s.cacheMapMu.Unlock()
		return tx.Hash(), nil
	}
}

type SendTxArgs struct {
	From     common.Address  `json:"from"`
	To       *common.Address `json:"to"`
	Gas      *hexutil.Uint64 `json:"gas"`
	GasPrice *hexutil.Big    `json:"gasPrice"`
	Value    *hexutil.Big    `json:"value"`
	Nonce    *hexutil.Uint64 `json:"nonce"`
	// We accept "data" and "input" for backwards-compatibility reasons. "input" is the
	// newer name and should be preferred by clients.
	Data *hexutil.Bytes `json:"data"`
}

func (args *SendTxArgs) setDefaults(ftl fractal) error {
	if args.Gas == nil {
		args.Gas = new(hexutil.Uint64)
		*(*uint64)(args.Gas) = 1e7
	}
	if args.GasPrice == nil {
		price := ftl.GasPrice()
		args.GasPrice = (*hexutil.Big)(price)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if args.Nonce == nil {
		nonce := ftl.TxPool().GetNonce(args.From, nil)
		args.Nonce = (*hexutil.Uint64)(&nonce)
	}
	if args.To == nil {
		// Contract creation
		var input []byte
		if args.Data != nil {
			input = *args.Data
		}
		if len(input) == 0 {
			return errors.New(`contract creation without any data provided`)
		}
	} else {
		args.Data = &hexutil.Bytes{}
	}
	return nil
}

type CallResult struct {
	Logs    []*types.Log
	GasUsed hexutil.Uint64
	//Print   string
}

func (s *TxPoolAPI) Call(args SendTxArgs) (CallResult, error) {
	defer func(start time.Time) { log.Debug("Executing WASM call finished", "runtime", time.Since(start)) }(time.Now())

	if args.To == nil {
		log.Info("Call: deploy contract")
	}

	block := s.ftl.BlockChain().CurrentBlock()
	stateDb, _ := s.ftl.BlockChain().StateAt(block.Header.StateHash)
	prevStateDb, _, _ := s.ftl.BlockChain().GetStateBeforeCacheHeight(block, uint8(params.ConfirmHeightDistance))

	err := args.setDefaults(s.ftl)
	if err != nil {
		return CallResult{}, err
	}

	msg := types.NewMessage(args.From, args.To, uint64(*args.Nonce), (*big.Int)(args.Value), uint64(*args.Gas), (*big.Int)(args.GasPrice), *args.Data, false)

	log.Info("TxPoolAPI Call", "data", hexutil.Encode(*args.Data), "from", args.From, "to", args.To)

	// Setup the gas pool (also for unmetered requests)
	// and apply the message.
	gp := new(types.GasPool).AddGas(math.MaxUint64)
	coinBase := s.ftl.Coinbase()
	stateDb.Prepare(common.Hash{}, 0, 0)
	callbackParamKey := wasm.GetGlobalRegisterParam().RegisterParam(stateDb, block)
	_, useGas, wasmFailed, err := txexec.WasmApplyMessage(prevStateDb, stateDb, msg, gp, s.ftl.BlockChain().GetChainConfig().MaxNonceBitLength, coinBase, callbackParamKey)
	wasm.GetGlobalRegisterParam().UnRegisterParam(callbackParamKey)
	if err != nil {
		if wasmFailed {
			log.Warn("TxPoolAPI Call: WASM execute failed", "err", err)
		} else {
			log.Error("TxPoolAPI Call: ApplyTransaction err", "from", msg.From(), "nonce", msg.Nonce(), "err", err)
			return CallResult{}, err
		}
	}

	logs := stateDb.GetLogs(common.Hash{})
	//print := "Wasm system print here."

	return CallResult{logs, hexutil.Uint64(useGas)}, err
}

//func (s *TxPoolAPI) GetReceipt(ctx context.Context, hash common.Hash) types.Receipts {
//	return s.ftl.GetReceipts(ctx, hash)
//}

func (s *TxPoolAPI) PendingTransactions() types.Transactions {
	return s.ftl.GetPoolTransactions()
}

func (s *TxPoolAPI) GasPrice() *hexutil.Big {
	return (*hexutil.Big)(s.ftl.GasPrice())
}

// TODO
//func (s *TxPoolAPI) SetGasPrice(gasPrice *hexutil.Big) {
//	s.ftl.gasPrice = (*big.Int)(gasPrice)
//}
