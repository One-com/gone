package metric

import "github.com/One-com/gone/metric/num64"

// Sink is a sink for metrics data which methods are guaranteed to be called
// synchronized. Thus it can keep state like buffers without locking
// metric Client should Record*() to a Sink.
// The idea is that the Sink in principle needs to be go-routine safe,
// but Sink.Record*() will be called synchronized and only from one go-routine, so you can spare
// the synchronization in the Record*() implementation it self if the Sink can
// be called upon to Clone a new instance for each go-routine using the sink.
type Sink interface {
	// Record a named value of any time with the Sink
	Record(mtype int, name string, value interface{})
	// RecordNumeric64 is a performance optimation for recording 64-bit values
	// using the num64 internal sub-package to create a "union" type for
	// int/uint/float 64 bit values.
	// Due to the nature of the metric event FIFO queue, using an interface{}
	// would have requires an additional heap-allocation due to escape analysis
	// not being able to guarantee the values can be stack allocated.
	RecordNumeric64(mtype int, name string, value num64.Numeric64)

	// Flush flushes the record values from Sink buffers.
	Flush()
}

// SinkFactory is the interface of objects returned to the user by sink implementations.
// It serves to be able for gone/metric to create new Sink objects which can be called.
// concurrently under external locking.
// The returned Sink is guaranteed to only be called from one go-routine under a lock.
// This allows a sink implementation to avoid using several layers of locks.
// A sink implementation can chose not to exploit this and a simple SinkFactory can
// just return it self as a Sink an do locking on all access.

type unlockedSink interface {
	UnlockedSink() Sink
}

// Flushers are created with this sink which just throws away data
// until a real sink is set.
// It's the user responsibility to not generate metrics before setting a sink if this
// is not wanted.
type nilSink struct{}

func (n *nilSink) Record(mtype int, name string, value interface{}) {
}

func (n *nilSink) RecordNumeric64(mtype int, name string, value num64.Numeric64) {
}

func (n *nilSink) Flush() {
}
