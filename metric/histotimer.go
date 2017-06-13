package metric

import (
	"github.com/One-com/gone/metric/num64"
	"time"
)

// A histogram is a series of int64 events all sent to the server
type Histogram eventStream

// NewHistogram creates a new persistent Histogram metric object with the default
// metric Client.
func NewHistogram(name string, opts ...MOption) *Histogram {
	return defaultClient.NewHistogram(name, opts...)
}

// NewHistogram creates a new persistent metric object measuring arbitrary sample values
// by allocating a client side FIFO buffer for recording and flushing measurements
func (c *Client) NewHistogram(name string, opts ...MOption) *Histogram {
	dequeuef := func(f Sink, val uint64) {
		n := num64.FromInt64(int64(val))
		f.RecordNumeric64(MeterHistogram, name, n)
	}
	t := c.newEventStream(name, MeterHistogram, dequeuef)
	c.register(t, opts...)
	return (*Histogram)(t)
}

// Sample record new event for the histogram
func (h *Histogram) Sample(d int64) {
	e := (*eventStream)(h)
	e.Enqueue(uint64(d))
}

// Timer is like Histogram, but the event is a time.Duration.
// values are remembered as milliseconds
type Timer eventStream

// NewTimer creates a new persistent Timer metric object with the default metric Client
func NewTimer(name string, opts ...MOption) *Timer {
	return defaultClient.NewTimer(name, opts...)
}

// NewTimer creates a new persistent metric object measuring timing values.
// by allocating a client side FIFO buffer for recording and flushing measurements
func (c *Client) NewTimer(name string, opts ...MOption) *Timer {
	dequeuef := func(f Sink, val uint64) {
		n := num64.FromUint64(val)
		f.RecordNumeric64(MeterTimer, name, n)
	}
	t := c.newEventStream(name, MeterTimer, dequeuef)
	c.register(t, opts...)
	return (*Timer)(t)
}

// Sample records a new duration event.
func (t *Timer) Sample(d time.Duration) {
	e := (*eventStream)(t)
	e.Enqueue(uint64(d.Nanoseconds() / int64(1000000)))
}
