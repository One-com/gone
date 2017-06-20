package metric

import (
	"errors"
	"github.com/One-com/gone/metric/num64"
	"sync"
	"time"
)

// This file is a shared implementation of two types of flushers: static/fixed or dynamic
// A flusher either flushes at a given interval, or when it's asked to do so by a Meter (which has filled its internal buffer)

// A flusher is either run with a fixed flushinterval with a go-routine which
// exits on stop(), or with a dynamic changeable flushinterval in a permanent go-routine.
// This is chosen by either calling run() og rundyn()
const (
	flusherTypeUndef = iota
	flusherTypeFixed
	flusherTypeDynamic // used for the defaultFlusher
)

type flusher struct {
	// Tell the flusher to exit - or (for defaultFlusher) restart
	stopChan chan struct{}
	// Kick the flusher to reconsider interval (used for defaultFlusher)
	kickChan chan struct{}

	// The flusher interval
	interval time.Duration

	// The Meters (metrics objects) being flushed by this flusher
	mu     sync.Mutex
	meters []Meter

	// only set once by the run/rundyn method to fix how the flusher is used.
	ftype int

	// The sink of data being flushed. Created from a SinkFactory.
	// The Sink is guaranteed to be called under an external lock, so it
	// doesn't need to use locking it self.
	sink Sink
}

func newFlusher(interval time.Duration) *flusher {
	f := &flusher{interval: interval, sink: &nilSink{}}
	f.stopChan = make(chan struct{})
	return f
}

func (f *flusher) setSink(sink Sink) {
	if sink == nil {
		return
	}
	f.mu.Lock()
	if sf, ok := sink.(unlockedSink); ok {
		f.sink = sf.UnlockedSink()
	} else {
		f.sink = sink
	}
	f.mu.Unlock()
}

func (f *flusher) stop() {
	f.stopChan <- struct{}{}
}

func (f *flusher) setInterval(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.ftype != flusherTypeFixed { // actually an assert
		if f.kickChan == nil {
			f.kickChan = make(chan struct{}, 1)
		}
		f.interval = d
		// book an interval consideration
		select {
		case f.kickChan <- struct{}{}:
		default:
			// already booked
		}
	}
}

// A go-routine which will flush at adjustable intervals and doesn't
// exit if interval is zero.
// This is used for the defaultFlusher of the Client
func (f *flusher) rundyn() {
	var interval time.Duration

	f.mu.Lock()
	if f.ftype == flusherTypeFixed {
		panic("Attempt to make fixed flusher dynamic")
	} else {
		f.ftype = flusherTypeDynamic
	}

	if f.kickChan == nil {
		f.kickChan = make(chan struct{}, 1)
	}
	f.mu.Unlock()

	var ticker *time.Ticker

RUNNING: // two cases - either with a flush or not
	for {
		f.mu.Lock()
		// take any new interval into account
		interval = f.interval
		f.mu.Unlock()
		if interval == 0 {
			// sit here waiting doing nothing
			select {
			case <-f.stopChan:
				break RUNNING
			case <-f.kickChan:
			}
		} else {
			ticker = time.NewTicker(interval)
		LOOP:
			for {
				select {
				case <-f.stopChan:
					ticker.Stop()
					break RUNNING
				case <-f.kickChan:
					ticker.Stop()
					break LOOP // to test to make a new ticker
				case <-ticker.C:
					f.Flush()
				}
			}
		}
	}
	f.Flush()
}

// Run the flusher until stopchan.
// The flusher is fixed to be a flusherTypeFixed.
func (f *flusher) run(done *sync.WaitGroup) {
	defer done.Done()

	f.mu.Lock()
	if f.ftype == flusherTypeDynamic {
		panic("Attempt to make default flusher fixed")
	} else {
		f.ftype = flusherTypeFixed
	}
	f.mu.Unlock()

	if f.interval == 0 {
		// don't start a meaningless flusher
		return
	}

	ticker := time.NewTicker(f.interval)
LOOP:
	for {
		select {
		case <-f.stopChan:
			ticker.Stop()
			break LOOP
		case <-ticker.C:
			f.Flush()
		}
	}
	f.Flush()
}

// flush a single meter. Sync with the Flusher mutex
func (f *flusher) FlushMeter(m Meter) {
	f.mu.Lock()
	m.FlushReading(f.sink)
	f.mu.Unlock()
}

// flush all meters. Sync with the Flusher mutex
func (f *flusher) Flush() {
	f.mu.Lock()
	for _, m := range f.meters {
		m.FlushReading(f.sink)
	}
	f.sink.Flush()
	f.mu.Unlock()
}

// Register a meter in the flusher. If the meters needs to know
// the flusher to do autoflushing, tell it.
func (f *flusher) register(m Meter) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.meters = append(f.meters, m)
	if a, ok := m.(autoFlusher); ok {
		a.setFlusher(f)
	}
}

var errDeregister = errors.New("Meter not known to client")

// deregister removes a meter from the list and keeps the list without empty slots.
func (f *flusher) deregister(m Meter) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	found := -1
	l := len(f.meters)
	for i, slot := range f.meters {
		if slot == m {
			found = i
		}
	}
	if found != -1 {
		if found < l-1 { // if l == 0 then found == -1
			// move the last slot here
			last := f.meters[l-1]
			f.meters[found] = last
		}
		f.meters = f.meters[:l-1] // clip of the end.
		return nil
	}
	return errDeregister
}

// Record records a value directly at the sink of this flusher.
// optionally flusing the sink.
func (f *flusher) Record(mtype int, name string, value interface{}, flush bool) {
	f.mu.Lock()
	f.sink.Record(mtype, name, value)
	if flush {
		f.sink.Flush()
	}
	f.mu.Unlock()
}

// RecordNumeric64 records a value directly at the sink of this flusher.
// optionally flusing the sink.
func (f *flusher) RecordNumeric64(mtype int, name string, value num64.Numeric64, flush bool) {
	f.mu.Lock()
	f.sink.RecordNumeric64(mtype, name, value)
	if flush {
		f.sink.Flush()
	}
	f.mu.Unlock()
}

// FlushSink asks the Sink attached to this flusher to flush it's internal buffers.
func (f *flusher) FlushSink() {
	f.mu.Lock()
	f.sink.Flush()
	f.mu.Unlock()
}
