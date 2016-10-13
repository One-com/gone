package metric

// Sink is a sink for metrics data which methods are guaranteed to be called
// synchronized. Thus it can keep state like buffers without locking
// metric Client should Record*() to a Sink.
// The idea is that the Sink in principle needs to be go-routine safe,
// but Sink.Record*() will be called synchronized, so you can spare
// the synchronization in the Record*() implementation it self if the Sink can
// be called upon to Clone a new instance for each go-routine using the sink.
type Sink interface {
	Record(mtype int, name string, value interface{})
	RecordNumeric64(mtype int, name string, value Numeric64)
	Flush()
}

// SinkFactory is the interface of objects returned to the user by sink implementations.
// It serves to be able for gone/metric to create new Sink objects which can be called.
// concurrently.
// This allows a sink implementation to avoid using several layers of locks.
type SinkFactory interface {
	Sink() Sink
}

// Flushers are created with this sink which just throws away data
// until a real sink is set.
// It's the user responsibility to not generate metrics before setting a sink if this
// is not wanted.
type nilSink struct{}
type nilSinkFactory struct{}

func (n *nilSink) Record(mtype int, name string, value interface{}) {
}

func (n *nilSink) RecordNumeric64(mtype int, name string, value Numeric64) {
}

func (n *nilSink) Flush() {
}

func (n *nilSinkFactory) Sink() Sink {
	return &nilSink{}
}

