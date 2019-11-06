package ftl

import (
	"context"
	"fmt"
	"math/big"
	"path"
	"strings"

	"github.com/fractal-platform/fractal/chain"
	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/crypto"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/ftl/api"
	"github.com/fractal-platform/fractal/ftl/network"
	"github.com/fractal-platform/fractal/ftl/protocol"
	ftl_sync "github.com/fractal-platform/fractal/ftl/sync"
	"github.com/fractal-platform/fractal/keys"
	"github.com/fractal-platform/fractal/logbloom/bloomquery"
	"github.com/fractal-platform/fractal/logbloom/bloomstorage"
	"github.com/fractal-platform/fractal/miner"
	"github.com/fractal-platform/fractal/p2p"
	"github.com/fractal-platform/fractal/packer"
	"github.com/fractal-platform/fractal/packer/pksvc"
	"github.com/fractal-platform/fractal/rpc/server"
	"github.com/fractal-platform/fractal/transaction/txexec"
	"github.com/fractal-platform/fractal/utils"
	"github.com/fractal-platform/fractal/utils/log"
)

// Fractal implements the Fractal full node service.
type Fractal struct {
	config *config.Config

	// Channel for shutting down the service
	shutdownChan chan bool // Channel for shutting down

	//
	signer types.Signer

	// chain database
	chainDb dbwrapper.Database

	// chain object
	blockchain    *chain.BlockChain
	bloomRequests chan chan *bloomquery.Retrieval // Channel receiving bloom data retrieval requests
	bloomIndexer  *bloomstorage.BloomIndexer      // Bloom indexer operating during block imports

	//
	txPool  pool.Pool
	pkgPool pool.Pool

	// key manager
	miningKeyManager *keys.MiningKeyManager
	packerKeyManager *keys.PackerKeyManager

	// for check point
	checkPointPriKey   crypto.PrivateKey
	checkPointNodeType types.CheckPointNodeTypeEnum

	//
	packer packer.Packer

	//
	miner    miner.Miner
	gasPrice *big.Int

	// for network
	protocolManager *network.ProtocolManager
	synchronizer    *ftl_sync.Synchronizer
	server          *p2p.Server // Currently running P2P networking layer

	// for rpc
	rpcServer      *rpcserver.Server
	adminRpcServer *rpcserver.Server
}

// New creates a new Fractal object.
func NewFtl(cfg *config.Config) (*Fractal, error) {
	var err error
	ftl := &Fractal{
		config:        cfg,
		shutdownChan:  make(chan bool),
		bloomRequests: make(chan chan *bloomquery.Retrieval),
		gasPrice:      common.Big1,
	}

	// setup signer
	ftl.signer = types.MakeSigner(ftl.config.ChainConfig.TxSignerType, ftl.config.ChainConfig.ChainID)

	// create database
	if cfg.NodeConfig.DataDir == "" {
		ftl.chainDb = dbwrapper.NewMemDatabase()
	} else {
		ftl.chainDb, err = dbwrapper.NewLDBDatabase(cfg.NodeConfig.ResolvePath("chaindata"), cfg.DatabaseCache, cfg.DatabaseHandles)
		if err != nil {
			log.Error("create leveldb failed", "error", err.Error())
			return nil, err
		}
	}

	// init the chain config
	cfg.ChainConfig, err = config.SetupChainConfig(ftl.chainDb, cfg.ChainConfig)
	if err != nil {
		log.Error("setup chain config failed", "error", err.Error())
		return nil, err
	}
	log.Info("Initialised chain configuration", "config", cfg.ChainConfig)

	// setup genesis block
	_, err = config.SetupGenesisBlock(ftl.chainDb, cfg.Genesis)
	if err != nil {
		log.Error("setup genesis block failed", "error", err.Error())
		return nil, err
	}

	// setup params about check point
	checkPointKeyFile := path.Join(ftl.config.NodeConfig.ResolvePath("keys"), "check_point_key.json")
	ftl.checkPointPriKey, err = keys.LoadCheckPointKey(checkPointKeyFile, ftl.config.CheckPointPriKeyPass)
	if err != nil {
		log.Info("Set check point node type: normal")
		ftl.checkPointNodeType = types.NormalNode
	} else {
		log.Info("Set check point node type: special")
		ftl.checkPointNodeType = types.SpecialNode
	}

	// create blockchain
	executor := txexec.NewExecutor(cfg.ChainConfig.TxExecutorType, cfg.ChainConfig.MaxNonceBitLength, ftl.signer)
	ftl.blockchain, err = chain.NewBlockChain(cfg, ftl.chainDb, executor, cfg.PackerInfoCacheSize, ftl.checkPointNodeType)
	if err != nil {
		log.Error("create blockchain failed", "error", err.Error())
		return nil, err
	}

	// setup bloom
	ftl.bloomIndexer = bloomstorage.NewBloomIndexer(ftl.chainDb)
	ftl.bloomIndexer.Start(ftl.blockchain)

	// setup pool
	if ftl.config.TxPoolConfig.Journal != "" {
		ftl.config.TxPoolConfig.Journal = cfg.NodeConfig.ResolvePath(ftl.config.TxPoolConfig.Journal)
	}
	if ftl.config.PkgPoolConfig.Journal != "" {
		ftl.config.PkgPoolConfig.Journal = cfg.NodeConfig.ResolvePath(ftl.config.PkgPoolConfig.Journal)
	}
	ftl.pkgPool = pool.NewPkgPool(*cfg.PkgPoolConfig, ftl.blockchain)
	ftl.txPool = pool.NewTxPool(ftl.config, ftl.blockchain)

	// setup keys for packer&miner
	ftl.packerKeyManager = keys.NewPackerKeyManager(ftl.config.PackerKeyFolder, ftl.config.KeyPass)
	if err := ftl.packerKeyManager.Start(); err != nil {
		return nil, err
	}

	ftl.miningKeyManager = keys.NewMiningKeyManager(ftl.config.MinerKeyFolder, ftl.config.KeyPass)
	if err := ftl.miningKeyManager.Start(); err != nil {
		return nil, err
	}

	// setup packer
	ftl.packer = pksvc.NewPacker(ftl.config, ftl.pkgPool, ftl.packerKeyManager, ftl.signer, ftl.blockchain, ftl.config.ChainConfig.PackerGroupSize)

	// setup miner
	ftl.miner = miner.NewFtlMiner(ftl.blockchain, executor, ftl.txPool, ftl.pkgPool, ftl.miningKeyManager)
	keys := ftl.miningKeyManager.Keys()
	for addr := range keys {
		log.Info("set coinbase", "coinbase", addr)
		ftl.miner.SetCoinbase(addr)
		break
	}
	if ftl.config.MinerEnable {
		ftl.miner.Start()
	}

	// setup protocol manager
	if ftl.protocolManager, err = network.NewProtocolManager(ftl.config.ChainConfig.ChainID, ftl.miner, ftl.blockchain, ftl.packer, ftl.txPool, ftl.pkgPool); err != nil {
		log.Error("create protocol manager failed", "error", err.Error())
		return nil, err
	}
	ftl.synchronizer = ftl_sync.NewSynchronizer(ftl.blockchain, ftl.miner, ftl.packer, ftl.protocolManager.RemovePeer, ftl.protocolManager.BlockProcessCh, ftl.config.SyncConfig)
	ftl.protocolManager.SetSynchronizer(ftl.synchronizer)
	log.Info("Initialising Fractal protocol", "versions", protocol.ProtocolVersions, "network", ftl.config.ChainConfig.ChainID)

	return ftl, nil
}

// starting all internal goroutines.
func (s *Fractal) Start() error {
	// start p2p server
	if err := s.startP2P(); err != nil {
		return err
	}

	// start rpc server
	s.startRPC()
	s.startAdminRPC()

	bloomquery.StartBloomHandlers(s.shutdownChan, s.bloomRequests, s.chainDb)

	// Start the networking layer
	s.protocolManager.Start(s.server.MaxPeers)
	return nil
}

// Start create a live P2P node and starts running it.
func (s *Fractal) startP2P() error {
	// Initialize the p2p server. This creates the node key and
	// discovery databases.
	serverConfig := s.config.NodeConfig.P2P
	serverConfig.PrivateKey = s.config.NodeConfig.NodeKey()
	serverConfig.Name = s.config.NodeConfig.NodeName()
	serverConfig.Logger = log.NewSubLogger()
	if serverConfig.StaticNodes == nil {
		serverConfig.StaticNodes = s.config.NodeConfig.StaticNodes()
	}
	if serverConfig.TrustedNodes == nil {
		serverConfig.TrustedNodes = s.config.NodeConfig.TrustedNodes()
	}
	if serverConfig.NodeDatabase == "" {
		serverConfig.NodeDatabase = s.config.NodeConfig.NodeDB()
	}
	running := &p2p.Server{Config: serverConfig}
	running.Protocols = append(running.Protocols, s.protocolManager.SubProtocols...)
	log.Info("Starting peer-to-peer node", "instance", serverConfig.Name)
	if err := running.Start(); err != nil {
		log.Error("start p2p server failed", "err", err.Error())
		return err
	}

	// Finish initializing the startup
	s.server = running
	return nil
}

// start rpc service
func (s *Fractal) startRPC() {
	// Gather all the possible APIs to surface
	apis := s.apiList()

	apiList := []rpcserver.RpcApi{}
	for _, api := range apis {
		if s.config.NodeConfig.RpcApiList == nil || len(s.config.NodeConfig.RpcApiList) == 0 || utils.Contains(api.Namespace, s.config.NodeConfig.RpcApiList) {
			apiList = append(apiList, rpcserver.RpcApi{
				Namespace: api.Namespace,
				Version:   api.Version,
				Service:   api.Service,
			})
		}
	}

	s.rpcServer = rpcserver.NewServer(s.config.NodeConfig.HTTPCors, s.config.NodeConfig.RpcEndpoint)
	s.rpcServer.RegisterApis(apiList)
	go s.rpcServer.ListenAndServe()
	log.Info("RPC endpoint opened", "endpoint", fmt.Sprintf("//%s", s.config.NodeConfig.RpcEndpoint))
}

// start admin rpc service
// TODO: use cmd param to config AdminRPC
func (s *Fractal) startAdminRPC() {
	// Gather all the possible APIs to surface
	apis := s.apiList()

	apiList := []rpcserver.RpcApi{}
	for _, api := range apis {
		if api.Namespace == "admin" {
			apiList = append(apiList, rpcserver.RpcApi{
				Namespace: api.Namespace,
				Version:   api.Version,
				Service:   api.Service,
			})
		}
	}

	s.adminRpcServer = rpcserver.NewServer(s.config.NodeConfig.HTTPCors, "127.0.0.1:8500")
	s.adminRpcServer.RegisterApis(apiList)
	go s.adminRpcServer.ListenAndServe()
	log.Info("RPC endpoint(for admin) opened", "endpoint", "127.0.0.1:8500")
}

// terminating all internal goroutines
func (s *Fractal) Stop() error {
	s.bloomIndexer.Close()
	s.protocolManager.Stop()
	s.txPool.Stop()
	s.miner.Stop()
	s.packer.StopPacking()

	s.miningKeyManager.Stop()
	s.packerKeyManager.Stop()

	s.chainDb.Close()
	close(s.shutdownChan)
	return nil
}

// APIs return the collection of RPC services.
// NOTE, some of these services probably need to be moved to somewhere else.
func (s *Fractal) apiList() []rpcserver.RpcApi {
	return []rpcserver.RpcApi{
		{
			Namespace: "admin",
			Version:   "1.0",
			Service:   api.NewAdminAPI(s.server, s),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewBlockChainAPI(s),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewFractalAPI(s),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewSynchronizerTestAPI(s),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewSynchronizerAPI(s),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewCheckPointAPI(s, s.checkPointPriKey),
		}, {
			Namespace: "ftl",
			Version:   "1.0",
			Service:   api.NewFilterAPI(s),
		}, {
			Namespace: "net",
			Version:   "1.0",
			Service:   api.NewNetAPI(s.server, s.NetVersion()),
		}, {
			Namespace: "pack",
			Version:   "1.0",
			Service:   api.NewPackerAPI(s.blockchain, s.packer),
		}, {
			Namespace: "txpool",
			Version:   "1.0",
			Service:   api.NewTxPoolAPI(s, s.config),
		},
	}
}

func (s *Fractal) MiningKeyManager() *keys.MiningKeyManager { return s.miningKeyManager }
func (s *Fractal) Coinbase() common.Address                 { return s.miner.GetCoinbase() }
func (s *Fractal) StartMining() error {
	go s.miner.Start()
	return nil
}
func (s *Fractal) StopMining()        { s.miner.Stop() }
func (s *Fractal) IsMining() bool     { return s.miner.IsMining() }
func (s *Fractal) Miner() miner.Miner { return s.miner }

func (s *Fractal) BlockChain() *chain.BlockChain        { return s.blockchain }
func (s *Fractal) Packer() packer.Packer                { return s.packer }
func (s *Fractal) Synchronizer() *ftl_sync.Synchronizer { return s.synchronizer }
func (s *Fractal) TxPool() pool.Pool                    { return s.txPool }
func (s *Fractal) ChainDb() dbwrapper.Database          { return s.chainDb }
func (s *Fractal) IsListening() bool                    { return true } // Always listening
func (s *Fractal) FtlVersion() int                      { return int(s.protocolManager.SubProtocols[0].Version) }
func (s *Fractal) NetVersion() uint64                   { return s.config.ChainConfig.ChainID }
func (s *Fractal) Config() *config.Config               { return s.config }
func (s *Fractal) Signer() types.Signer                 { return s.signer }
func (s *Fractal) GasPrice() *big.Int                   { return s.gasPrice }

func (s *Fractal) GetPoolTransactions() types.Transactions {
	content := s.txPool.Content()
	var txs types.Transactions
	for _, batch := range content {
		for _, tx := range batch {
			txs = append(txs, tx.(*types.Transaction))
		}
	}
	return txs
}

func (s *Fractal) GetBlock(ctx context.Context, fullHash common.Hash) *types.Block {
	return s.blockchain.GetBlock(fullHash)
}

func (s *Fractal) CurrentBlock(ctx context.Context) *types.Block {
	return s.blockchain.CurrentBlock()
}

func (s *Fractal) GetMainBranchBlock(height uint64) (*types.Block, error) {
	return s.blockchain.GetMainBranchBlock(height)
}

func (s *Fractal) GetBlockStr(blockStr string) *types.Block {
	if strings.ToLower(blockStr) == "latest" {
		return s.blockchain.CurrentBlock()
	} else {
		blockHash := common.HexToHash(blockStr)
		return s.blockchain.GetBlock(blockHash)
	}
}

func (s *Fractal) GetReceipts(ctx context.Context, fullHash common.Hash) types.Receipts {
	return dbaccessor.ReadReceipts(s.chainDb, fullHash)
}

func (s *Fractal) GetLogs(ctx context.Context, fullHash common.Hash) [][]*types.Log {
	receipts := dbaccessor.ReadReceipts(s.chainDb, fullHash)
	if receipts == nil {
		return nil
	}
	logs := make([][]*types.Log, len(receipts))
	for i, receipt := range receipts {
		logs[i] = receipt.Logs
	}
	return logs
}

func (b *Fractal) BloomRequestsReceiver() chan chan *bloomquery.Retrieval {
	return b.bloomRequests
}

func (b *Fractal) SubscribeInsertBloomEvent(ch chan<- types.BloomInsertEvent) event.Subscription {
	return b.bloomIndexer.SubscribeInsertBloomEvent(ch)
}
