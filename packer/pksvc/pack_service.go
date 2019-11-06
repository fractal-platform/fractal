package pksvc

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fractal-platform/fractal/common"
	"github.com/fractal-platform/fractal/core/config"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/core/pool"
	"github.com/fractal-platform/fractal/core/types"
	"github.com/fractal-platform/fractal/event"
	"github.com/fractal-platform/fractal/packer/tx_collector"
	"github.com/fractal-platform/fractal/utils/log"
)

type packService struct {
	txCollector *tx_collector.TxCollector
	txChan      chan types.Transactions
	container   *txContainer
	pkgPool     pool.Pool
	worker      *worker
	chain       blockChain

	nonces map[common.Address]uint64

	running int32
}

func newPackService(cfg *config.Config, packerKeyManager packerKeyManager, pkgPool pool.Pool, txSigner types.Signer, chain blockChain, packerGroupSize uint64) *packService {
	fakeMode := cfg.FakeMode
	interval := cfg.PackerInterval
	listenAddr := cfg.PackerCollectAddr
	container := newTxContainer(fakeMode, make(chan *types.Transaction), txSigner, chain, packerGroupSize);
	p := &packService{
		txCollector: tx_collector.NewTxCollector(listenAddr),
		container:   container,
		pkgPool:     pkgPool,
		chain:       chain,
		nonces:      make(map[common.Address]uint64),
	}
	p.worker = newWorker(fakeMode, interval, txSigner, packerKeyManager, p, container, new(event.Feed), chain, packerGroupSize)
	atomic.StoreInt32(&p.running, 0)

	// if the param<PackerEnable> is true, the packing service is enabled by default.
	if cfg.PackerEnable {
		p.StartPacking(cfg.PackerId)
	}

	return p
}

func (self *packService) InsertTransactions(txs types.Transactions) []error {
	if !self.IsPacking() {
		return []error{errors.New("packer service has not been started")}
	}
	return self.container.AddAll(txs)
}

func (self *packService) StartPacking(packerIndex uint32) {
	if self.IsPacking() {
		return
	}
	log.Info("start packing", "packerIndex", packerIndex)
	// enable receive transactions
	self.container.SetPackerIndex(packerIndex)
	self.txChan = make(chan types.Transactions)
	self.worker.start(packerIndex)
	go func() {
		self.txCollector.Start(self.txChan)
		for {
			txs, ok := <-self.txChan
			if !ok {
				return
			}
			self.container.AddAll(txs)
		}
	}()

	atomic.StoreInt32(&self.running, 1)
}

func (self *packService) StopPacking() {
	if !self.IsPacking() {
		return
	}
	atomic.StoreInt32(&self.running, 0)
	self.txCollector.Stop()
	self.worker.stop()
}

func (self *packService) IsPacking() bool {
	return atomic.LoadInt32(&self.running) == 1
}

func (self *packService) Subscribe(ch chan<- types.TxPackages) event.Subscription {
	return self.worker.newPkgEventFeed.Subscribe(ch)
}

// TODO: add param for no-pool-add
func (self *packService) InsertRemoteTxPackage(pkg *types.TxPackage) error {
	self.chain.InsertTxPackage(pkg)
	err := self.pkgPool.AddRemote(pkg)

	return err
}

func (self *packService) insertLocalTxPackage(pkg *types.TxPackage) error {
	self.chain.InsertTxPackage(pkg)
	err := self.pkgPool.AddLocal(pkg)

	return err
}

func (self *packService) FetchTxPackageFromPool() []*types.TxPackage {
	ret := make([]*types.TxPackage, 0)
	content := self.pkgPool.Content()
	for _, value := range content {
		for _, ele := range value {
			ret = append(ret, ele.(*types.TxPackage))
		}
	}
	return ret
}

func (self *packService) GetAndIncNonce(addr common.Address) uint64 {
	var nonce uint64
	if self.nonces[addr] == 0 {
		nonce, _ = dbaccessor.ReadPackerNonce(self.chain.Database(), addr)
	} else {
		nonce = self.nonces[addr]
	}
	self.nonces[addr] = nonce + 1
	dbaccessor.WritePackerNonce(self.chain.Database(), addr, nonce+1)
	return nonce
}

type worker struct {
	packerIndex *uint32

	fakeMode         bool
	txSigner         types.Signer
	pkgSigner        types.PkgSigner
	packerKeyManager packerKeyManager
	packService      *packService

	txContainer     *txContainer
	chain           blockChain
	packerGroupSize uint64

	interval int
	timeout  *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup // for shutdown sync

	newPkgEventFeed *event.Feed
}

func newWorker(fakeMode bool, interval int, txSigner types.Signer, packerKeyManager packerKeyManager, packService *packService, txContainer *txContainer, newPkgEventFeed *event.Feed, chain blockChain, packerGroupSize uint64) *worker {
	w := &worker{
		fakeMode:         fakeMode,
		txSigner:         txSigner,
		pkgSigner:        types.MakePkgSigner(fakeMode),
		packerKeyManager: packerKeyManager,
		packService:      packService,
		txContainer:      txContainer,
		chain:            chain,
		packerGroupSize:  packerGroupSize,
		interval:         interval,
		newPkgEventFeed:  newPkgEventFeed,
	}
	return w
}

func (w *worker) start(packerIndex uint32) {
	w.packerIndex = &packerIndex
	w.timeout = time.NewTicker(time.Duration(w.interval) * time.Second)
	w.ctx, w.cancel = context.WithCancel(context.Background())
	go w.loop()
}

func (w *worker) stop() {
	if w.cancel != nil {
		w.cancel()
		w.wg.Wait()
		log.Info("worker of packer service is stopped")
	}
	if w.timeout != nil {
		w.timeout.Stop()
	}
	w.packerIndex = nil
}

func (w *worker) loop() {
	w.wg.Add(1)
	defer w.wg.Done()

	for {
		select {
		case <-w.txContainer.newTxCh:
			if w.txContainer.Count() > DefaultPkgSize {
				if txs, err := w.txContainer.Pop(DefaultPkgSize); err == nil {
					w.packAndSave(txs)
				}
			}
		case <-w.timeout.C:
			if txs, err := w.txContainer.Pop(w.txContainer.Count()); err == nil && len(txs) > 0 {
				w.packAndSave(txs)
			}
		case <-w.ctx.Done():
			return
		}
	}
}

func (w *worker) packAndSave(txs []*types.Transaction) error {
	var txPackage *types.TxPackage
	var err error
	if txPackage, err = w.pack(txs); err != nil {
		log.Error("worker pack error", "err", err)
		return err
	}

	if err = w.packService.insertLocalTxPackage(txPackage); err != nil {
		log.Error("worker save error", "err", err)
		return err
	}

	log.Info("worker pack success")
	return nil
}

func (w *worker) pack(txs []*types.Transaction) (*types.TxPackage, error) {
	// reValidate before packing
	var removes []int
	currentBlock := w.chain.CurrentBlock()
	if currentBlock == nil {
		return nil, pool.ErrBlockNotFound
	}

	// Do another check, because the packer information may have changed during the period from receiving transaction to packing transaction.
	for i, tx := range txs {
		if !tx.MatchPacker(w.packerGroupSize, *w.packerIndex, w.txSigner) {
			removes = append(removes, i)
		}
	}
	if len(removes) > 0 {
		var tmp []*types.Transaction
		var num = len(removes)
		tmp = append(tmp, txs[0:removes[0]]...)
		for i := 0; i < num-1; i++ {
			tmp = append(tmp, txs[removes[i]+1:removes[i]]...)
		}
		tmp = append(tmp, txs[removes[num-1]+1:]...)
		txs = tmp
	}
	log.Info("Worker pack transactions", "remove num", len(removes), "pack num", len(txs))

	// pack
	packerInfo, block, err := w.chain.GetPrePackerInfoByIndex(currentBlock, *w.packerIndex)
	if err != nil {
		return nil, err
	}
	privateKey, err := w.packerKeyManager.GetPrivateKey(packerInfo.Coinbase, packerInfo.PackerPubKey)
	if err != nil {
		return nil, err
	}

	pkg := types.NewTxPackage(packerInfo.Coinbase, w.packService.GetAndIncNonce(packerInfo.Coinbase), txs, block.FullHash(), uint64(time.Now().UnixNano()/1e6))
	log.Debug("[pksvc][worker.pack()]new package", "pkg", pkg)

	var newPkg *types.TxPackage
	if !w.fakeMode {
		var err error
		newPkg, err = w.pkgSigner.Sign(pkg, privateKey)
		if err != nil {
			log.Debug("[pksvc][worker.pack()]error happened during signing pkg", "err", err)
			return nil, err
		}
	} else {
		newPkg = pkg
	}

	newPkg.ReceivedAt = time.Now()
	log.Info("Generate a new tx package", "pkgHash", newPkg.Hash(), "blockHash", newPkg.BlockFullHash(), "txCount", len(newPkg.Transactions()))
	go w.newPkgEventFeed.Send(types.TxPackages{newPkg})
	return newPkg, nil
}
