package downloader

import (
	"sync"
	"time"

	"github.com/fractal-platform/fractal/utils/log"
)

// common part for block_fetcher&package_fetcher
type fetcher struct {
	peers      *peersManager
	repCh      chan dataPack
	cancel     chan struct{}
	done       chan struct{}
	err        error
	cancelOnce sync.Once
}

type fetcherReq struct {
	timeout time.Duration // Maximum Round trip time for this to complete
	timer   *time.Timer   // Timer to fire when the RTT timeout expires
	peer    *Peer
	dropped bool
}

// Cancel cancels the fetcher and waits until it has shut down.
func (f *fetcher) Cancel() error {
	f.cancelOnce.Do(func() { close(f.cancel) })
	return f.Wait()
}

// Wait blocks until the fetcher is done or canceled.
func (f *fetcher) Wait() error {
	<-f.done
	log.Info("fetcher wait returns", "err", f.err)
	return f.err
}

// deliver delives the data to a fetcher.
func (f *fetcher) deliver(packet dataPack) (err error) {
	f.repCh <- packet
	return nil
}
