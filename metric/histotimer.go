package metric

import (
	"github.com/One-com/gone/metric/num64"
	"time"
)

// Histogram is a series of int64 events all sent to the server
type Histogram struct {
	*eventStream
}

// NewHistogram creates a new persistent metric object measuring arbitrary sample values
// by allocating a client side FIFO buffer for recording and flushing measurements
func NewHistogram(name string) Histogram {
	dequeuef := func(f Sink, val uint64) {
		n := num64.FromInt64(int64(val))
		f.RecordNumeric64(MeterHistogram, name, n)
	}
	t := newEventStream(name, dequeuef)
	return Histogram{t}
}

// Sample record new event for the histogram
func (e Histogram) Sample(d int64) {
	//e := (*eventStream)(h)
	e.enqueue(uint64(d))
}

// Timer is like Histogram, but the event is a time.Duration.
// values are remembered as milliseconds
type Timer struct {
	*eventStream
}

// NewTimer creates a new persistent metric object measuring timing values.
// by allocating a client side FIFO buffer for recording and flushing measurements
func NewTimer(name string) Timer {
	dequeuef := func(f Sink, val uint64) {
		n := num64.FromUint64(val)
		f.RecordNumeric64(MeterTimer, name, n)
	}
	t := newEventStream(name, dequeuef)
	return Timer{t}
}

// Sample records a new duration event.
func (e Timer) Sample(d time.Duration) {
	//e := (*eventStream)(t)
	e.enqueue(uint64(d.Nanoseconds() / int64(1000000)))
}
