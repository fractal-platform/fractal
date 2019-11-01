package metrics

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	ds "github.com/fractal-platform/fractal/utils/datastructure"
	"github.com/rcrowley/go-metrics"
)

// StandardMA is the standard implementation of an MA and tracks the number
// of uncounted events and processes them on each tick.  It uses the
// sync/atomic package to manage uncounted events.
type StandardMA struct {
	uncounted    int64 // /!\ this should be the first member to ensure 64-bit alignment
	rate         uint64
	instantRates *ds.LimitQueue
	init         uint32
	mutex        sync.Mutex
}

func NewMA1() *StandardMA {
	ma := &StandardMA{
		instantRates: ds.NewLimitQueue(12),
	}

	ma.instantRates.Add(0.0)
	return ma
}

func NewMA5() *StandardMA {
	ma := &StandardMA{
		instantRates: ds.NewLimitQueue(60),
	}
	ma.instantRates.Add(0.0)
	return ma
}

func NewMA15() *StandardMA {
	ma := &StandardMA{
		instantRates: ds.NewLimitQueue(180),
	}
	ma.instantRates.Add(0.0)
	return ma
}

// Rate returns the moving average rate of events per second.
func (a *StandardMA) Rate() float64 {
	currentRate := math.Float64frombits(atomic.LoadUint64(&a.rate)) * float64(1e9)
	return currentRate
}

// Tick ticks the clock to update the moving average.  It assumes it is called
// every five seconds.
func (a *StandardMA) Tick() {
	// Optimization to avoid mutex locking in the hot-path.
	if atomic.LoadUint32(&a.init) == 1 {
		a.updateRate(a.fetchInstantRate())
	} else {
		// Slow-path: this is only needed on the first Tick() and preserves transactional updating
		// of init and rate in the else block. The first conditional is needed below because
		// a different thread could have set a.init = 1 between the time of the first atomic load and when
		// the lock was acquired.
		a.mutex.Lock()
		if atomic.LoadUint32(&a.init) == 1 {
			// The fetchInstantRate() uses atomic loading, which is unecessary in this critical section
			// but again, this section is only invoked on the first successful Tick() operation.
			a.updateRate(a.fetchInstantRate())
		} else {
			atomic.StoreUint32(&a.init, 1)
			atomic.StoreUint64(&a.rate, math.Float64bits(a.fetchInstantRate()))
		}
		a.mutex.Unlock()
	}
}

func (a *StandardMA) fetchInstantRate() float64 {
	count := atomic.LoadInt64(&a.uncounted)
	atomic.AddInt64(&a.uncounted, -count)
	instantRate := float64(count) / float64(5e9)
	return instantRate
}

func (a *StandardMA) updateRate(instantRate float64) {
	currentRate := math.Float64frombits(atomic.LoadUint64(&a.rate))
	//fmt.Printf("updateRate(%d): %f, %f, %f\n", a.instantRates.Capacity(), instantRate * 1e9, a.instantRates.Peek().(float64) * 1e9, currentRate * 1e9)
	currentRate += (instantRate - a.instantRates.Peek().(float64)) / float64(a.instantRates.Capacity())
	atomic.StoreUint64(&a.rate, math.Float64bits(currentRate))
	a.instantRates.Add(instantRate)
}

// Update adds n uncounted events.
func (a *StandardMA) Update(n int64) {
	atomic.AddInt64(&a.uncounted, n)
}

// MAMeterSnapshot is a read-only copy of another Meter.
type MAMeterSnapshot struct {
	count                          int64
	rate1, rate5, rate15, rateMean uint64
}

// Count returns the count of events at the time the snapshot was taken.
func (m *MAMeterSnapshot) Count() int64 { return m.count }

// Mark panics.
func (*MAMeterSnapshot) Mark(n int64) {
	panic("Mark called on a MAMeterSnapshot")
}

// Rate1 returns the one-minute moving average rate of events per second at the
// time the snapshot was taken.
func (m *MAMeterSnapshot) Rate1() float64 { return math.Float64frombits(m.rate1) }

// Rate5 returns the five-minute moving average rate of events per second at
// the time the snapshot was taken.
func (m *MAMeterSnapshot) Rate5() float64 { return math.Float64frombits(m.rate5) }

// Rate15 returns the fifteen-minute moving average rate of events per second
// at the time the snapshot was taken.
func (m *MAMeterSnapshot) Rate15() float64 { return math.Float64frombits(m.rate15) }

// RateMean returns the meter's mean rate of events per second at the time the
// snapshot was taken.
func (m *MAMeterSnapshot) RateMean() float64 { return math.Float64frombits(m.rateMean) }

// Snapshot returns the snapshot.
func (m *MAMeterSnapshot) Snapshot() metrics.Meter { return m }

// Stop is a no-op.
func (m *MAMeterSnapshot) Stop() {}

// StandardMaMeter is the standard ma implementation of a Meter.
type StandardMAMeter struct {
	// Only used on stop.
	lock        sync.Mutex
	snapshot    *MAMeterSnapshot
	a1, a5, a15 *StandardMA
	startTime   time.Time
	stopped     uint32
}

// NewMeter constructs and registers a new StandardMeter and launches a
// goroutine.
// Be sure to unregister the meter from the registry once it is of no use to
// allow for garbage collection.
func NewRegisteredMAMeter(name string, r metrics.Registry) metrics.Meter {
	c := NewMAMeter()
	if nil == r {
		r = metrics.DefaultRegistry
	}
	err := r.Register(name, c)
	if err != nil {
		return nil
	}
	return c
}

// NewMAMeter constructs a new StandardMAMeter and launches a goroutine.
// Be sure to call Stop() once the meter is of no use to allow for garbage collection.
func NewMAMeter() metrics.Meter {
	if metrics.UseNilMetrics {
		return metrics.NilMeter{}
	}
	m := newMAMeter()
	arbiter.Lock()
	defer arbiter.Unlock()
	arbiter.meters[m] = struct{}{}
	if !arbiter.started {
		arbiter.started = true
		go arbiter.tick()
	}
	return m
}

func newMAMeter() *StandardMAMeter {
	return &StandardMAMeter{
		snapshot:  &MAMeterSnapshot{},
		a1:        NewMA1(),
		a5:        NewMA5(),
		a15:       NewMA15(),
		startTime: time.Now(),
	}
}

// Stop stops the meter, Mark() will be a no-op if you use it after being stopped.
func (m *StandardMAMeter) Stop() {
	m.lock.Lock()
	stopped := m.stopped
	m.stopped = 1
	m.lock.Unlock()
	if stopped != 1 {
		arbiter.Lock()
		delete(arbiter.meters, m)
		arbiter.Unlock()
	}
}

// Count returns the number of events recorded.
func (m *StandardMAMeter) Count() int64 {
	return atomic.LoadInt64(&m.snapshot.count)
}

// Mark records the occurance of n events.
func (m *StandardMAMeter) Mark(n int64) {
	if atomic.LoadUint32(&m.stopped) == 1 {
		return
	}

	atomic.AddInt64(&m.snapshot.count, n)

	m.a1.Update(n)
	m.a5.Update(n)
	m.a15.Update(n)
	m.updateSnapshot()
}

// Rate1 returns the one-minute moving average rate of events per second.
func (m *StandardMAMeter) Rate1() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.snapshot.rate1))
}

// Rate5 returns the five-minute moving average rate of events per second.
func (m *StandardMAMeter) Rate5() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.snapshot.rate5))
}

// Rate15 returns the fifteen-minute moving average rate of events per second.
func (m *StandardMAMeter) Rate15() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.snapshot.rate15))
}

// RateMean returns the meter's mean rate of events per second.
func (m *StandardMAMeter) RateMean() float64 {
	return math.Float64frombits(atomic.LoadUint64(&m.snapshot.rateMean))
}

// Snapshot returns a read-only copy of the meter.
func (m *StandardMAMeter) Snapshot() metrics.Meter {
	copiedSnapshot := MAMeterSnapshot{
		count:    atomic.LoadInt64(&m.snapshot.count),
		rate1:    atomic.LoadUint64(&m.snapshot.rate1),
		rate5:    atomic.LoadUint64(&m.snapshot.rate5),
		rate15:   atomic.LoadUint64(&m.snapshot.rate15),
		rateMean: atomic.LoadUint64(&m.snapshot.rateMean),
	}
	return &copiedSnapshot
}

func (m *StandardMAMeter) updateSnapshot() {
	rate1 := math.Float64bits(m.a1.Rate())
	rate5 := math.Float64bits(m.a5.Rate())
	rate15 := math.Float64bits(m.a15.Rate())
	rateMean := math.Float64bits(float64(m.Count()) / time.Since(m.startTime).Seconds())
	//fmt.Printf("rate1: %f\n", m.a1.Rate())

	atomic.StoreUint64(&m.snapshot.rate1, rate1)
	atomic.StoreUint64(&m.snapshot.rate5, rate5)
	atomic.StoreUint64(&m.snapshot.rate15, rate15)
	atomic.StoreUint64(&m.snapshot.rateMean, rateMean)
}

func (m *StandardMAMeter) tick() {
	m.a1.Tick()
	m.a5.Tick()
	m.a15.Tick()
	m.updateSnapshot()
}

// meterArbiter ticks meters every 5s from a single goroutine.
// meters are references in a set for future stopping.
type meterArbiter struct {
	sync.RWMutex
	started bool
	meters  map[*StandardMAMeter]struct{}
	ticker  *time.Ticker
}

var arbiter = meterArbiter{ticker: time.NewTicker(5e9), meters: make(map[*StandardMAMeter]struct{})}

// Ticks meters on the scheduled interval
func (ma *meterArbiter) tick() {
	for range ma.ticker.C {
		ma.tickMeters()
	}
}

func (ma *meterArbiter) tickMeters() {
	ma.RLock()
	defer ma.RUnlock()
	for meter := range ma.meters {
		meter.tick()
	}
}
