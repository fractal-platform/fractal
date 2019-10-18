package bloomquery

import (
	"github.com/fractal-platform/fractal/common/bitutil"
	"github.com/fractal-platform/fractal/core/dbaccessor"
	"github.com/fractal-platform/fractal/dbwrapper"
	"github.com/fractal-platform/fractal/params"
)

// bloomServiceThreads is the number of goroutines used globally to service bloombits
// lookups for all running filters when a Fractal instance start.
const bloomServiceThreads = 16

// StartBloomHandlers starts a batch of goroutines to accept bloom bit database
// retrievals from possibly a range of filters and serving the data to satisfy.
func StartBloomHandlers(shutdownChan chan bool, bloomRequests chan chan *Retrieval, chainDb dbwrapper.Database) {
	for i := 0; i < bloomServiceThreads; i++ {
		go func() {
			for {
				select {
				case <-shutdownChan:
					return

				case request := <-bloomRequests:
					task := <-request
					task.Bitsets = make([][]byte, len(task.Sections))
					for i, section := range task.Sections {
						if compVector, err := dbaccessor.ReadBloomBits(chainDb, task.Bit, section); err == nil {
							if blob, err := bitutil.DecompressBytes(compVector, int(params.BloomByteSize)); err == nil {
								task.Bitsets[i] = blob
							} else {
								task.Error = err
							}
						} else {
							task.Error = err
						}
					}
					request <- task
				}
			}
		}()
	}
}
