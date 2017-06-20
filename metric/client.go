package metric

import (
	"github.com/One-com/gone/metric/num64"
	"sync"
	"time"
)

// Client is the main object the applications holds to do metrics.
// It can be used directly for ad-hoc events, or be used to create persistent
// gauge/conter/timer/... objects optimised for bulk metric generation.
// A default metric Client with no FlushInterval set is Start()'ed by init()
type Client struct {

	// Wait for all flushers to empty before exiting Stop()
	done *sync.WaitGroup

	// protects the flusher map and the running flag.
	fmu sync.Mutex
	// a map of flushers doing flushing at given intervals
	// - to reuse a flusher for metrics with same interval
	flushers map[time.Duration]*flusher
	meters   map[Meter]*flusher

	// The flusher handling meters which have been given no specific
	// flush interval
	defaultFlusher *flusher

	// Factory for generating new independent sink objects for flushers.
	sinkf Sink

	running bool
}

// A single global client. Remember to set a default sink
var defaultClient *Client

func init() {
	defaultClient = NewClient(nil)
	defaultClient.Start()
}

// Default returns the default metric Client
func Default() *Client {
	return defaultClient
}

// NewClient returns you a client handle directly if you do not want to use the
// global default client.
// This creates a new metric client with a factory object for the sink.
// If sink == nil, the client will not emit metrics until a Sink is set using
// SetSink.
func NewClient(sink Sink, opts ...MOption) (client *Client) {

	client = &Client{}
	client.done = new(sync.WaitGroup)
	client.flushers = make(map[time.Duration]*flusher)
	client.meters = make(map[Meter]*flusher)

	// create a default flusher which do not flush by it self.
	client.defaultFlusher = newFlusher(0)

	client.SetSink(sink)

	client.SetOptions(opts...)

	return
}

// SetDefaultOptions sets options on the default metric client
func SetDefaultOptions(opts ...MOption) {
	c := defaultClient
	c.SetOptions(opts...)
}

// SetOptions sets options on a client - like the flush interval for metrics which
// haven't them selves a fixed flush interval
func (c *Client) SetOptions(opts ...MOption) {
	conf := MConfig{make(map[string]interface{})}
	for _, o := range opts {
		o(conf)
	}

	c.fmu.Lock()
	if f, ok := conf.cfg["flushInterval"]; ok {
		if d, ok := f.(time.Duration); ok {
			c.defaultFlusher.setInterval(d)
		}
	}
	c.fmu.Unlock()
}

// SetDefaultSink sets the sink for the default metics client
// The default client has no sink set initially. You need to set
// it calling this function.
func SetDefaultSink(sink Sink) {
	c := defaultClient
	c.SetSink(sink)
}

// SetSink sets the Sink factory of the client.
// You'll need to set a sink before any metrics will be emitted.
func (c *Client) SetSink(sink Sink) {
	c.fmu.Lock()

	c.sinkf = sink
	c.defaultFlusher.setSink(sink)
	for _, f := range c.flushers {
		if sink == nil {
			f.setSink(&nilSink{})
		} else {
			f.setSink(sink)
		}
	}
	c.fmu.Unlock()
}

// Start the default Client if stopped.
func Start() {
	defaultClient.Start()
}

// Start a stopped client
func (c *Client) Start() {
	c.fmu.Lock()
	defer c.fmu.Unlock()

	if c.running {
		return
	}

	go c.defaultFlusher.rundyn()

	for _, f := range c.flushers {
		c.done.Add(1)
		f.run(c.done)
	}

	c.running = true
}

// Stop the global default metrics Client
func Stop() {
	defaultClient.Stop()
}

// Stop a Client from flushing data.
// If any autoFlusher meters are still in use they will still flush when overflown.
func (c *Client) Stop() {
	c.fmu.Lock()
	defer c.fmu.Unlock()

	if !c.running {
		return
	}

	c.defaultFlusher.stop()

	for _, f := range c.flushers {
		f.stop()
	}
	c.done.Wait()
	c.running = false
}

// Deregister detaches a Meter (gauge/counter/timer...) from the client.
// It will no longer be flushed.
// An error is returned if the Meter was not registered.
func (c *Client) Deregister(m Meter) error {
	c.fmu.Lock()
	defer c.fmu.Unlock()
	if f, ok := c.meters[m]; ok {
		return f.deregister(m)

	}
	return errDeregister
}

// Register a Meter with the client, finding a flusher
// with the appropriate interval, if possible, else create a new flusher.
func (c *Client) Register(m Meter, opts ...MOption) {
	c.fmu.Lock()

	var f *flusher
	var flush time.Duration

	conf := MConfig{make(map[string]interface{})}
	for _, o := range opts {
		o(conf)
	}

	if fi, ok := conf.cfg["flushInterval"]; ok {
		flush = fi.(time.Duration)
		if f, ok = c.flushers[flush]; !ok {
			f = newFlusher(flush)
			f.setSink(c.sinkf)
			c.flushers[flush] = f
			if c.running {
				c.done.Add(1)
				go f.run(c.done)
			}
		}
		f.register(m)
		c.meters[m] = f
	} else {
		c.defaultFlusher.register(m)
		c.meters[m] = c.defaultFlusher
	}

	c.fmu.Unlock()
}

// Flush calls Flush() on the default client.
func Flush() {
	defaultClient.Flush()
}

// Flush the client default so no data is left in any pipeline buffers.
func (c *Client) Flush() {
	c.fmu.Lock()
	defer c.fmu.Unlock()
	for _, f := range c.flushers {
		f.Flush()
		f.FlushSink()
	}
	c.defaultFlusher.Flush()
	c.defaultFlusher.FlushSink()
}

//--------------------------------------------------------------

func (c *Client) RegisterCounter(name string, opts ...MOption) *Counter {
	meter := NewCounter(name)
	c.Register(meter, opts...)
	return meter
}

func (c *Client) RegisterGauge(name string, opts ...MOption) *GaugeUint64 {
	meter := NewGauge(name)
	c.Register(meter, opts...)
	return meter
}

func (c *Client) RegisterTimer(name string, opts ...MOption) Timer {
	meter := NewTimer(name)
	c.Register(meter, opts...)
	return meter
}

func (c *Client) RegisterHistogram(name string, opts ...MOption) Histogram {
	meter := NewHistogram(name)
	c.Register(meter, opts...)
	return meter
}

//--------------------------------------------------------------

// AdhocCounter creates an ad-hoc counter metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) AdhocCount(name string, val int, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterCounter, name, num64.FromInt64(int64(val)), flush)
}

// AdhocGauge creates an ad-hoc gauge metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) AdhocGauge(name string, val uint64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterGauge, name, num64.FromUint64(val), flush)
}

// AdhocTimer creates an ad-hoc timer metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) AdhocTime(name string, d time.Duration, flush bool) {
	val := d.Nanoseconds() / int64(1000000)
	c.defaultFlusher.RecordNumeric64(MeterTimer, name, num64.FromInt64(int64(val)), flush)
}

// AdhocSample creates an ad-hoc histogram metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) AdhocSample(name string, val int64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, num64.FromInt64(int64(val)), flush)
}

// Mark - send a ad-hoc zero histogram event immediately to allow the server side to indicate a unique event happened. This equivalent to calling Sample(name, 0, true) and can be used as a poor mans way to make qualitative events to be marked in the overall view of metrics. Like "process restart". Graphical views might allow you to draw these as special marks. For some sinks (like statsd) there's not dedicated way to send such events.
// Mark is equivalent to AdhocSample(name, 0, true)
func (c *Client) Mark(name string) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, num64.FromInt64(int64(0)), true)
}

//--------------------------------------------------------------

// AdhocCounter creates an ad-hoc counter metric event at the default client.
// If flush is true, the sink will be instructed to flush data immediately
func AdhocCount(name string, val int, flush bool) {
	defaultClient.AdhocCount(name, val, flush)
}

// AdhocGauge creates an ad-hoc gauge metric event at the default client.
// If flush is true, the sink will be instructed to flush data immediately
func AdhocGauge(name string, val uint64, flush bool) {
	defaultClient.AdhocGauge(name, val, flush)
}

// AdhocTimer creates an ad-hoc timer metric event at the default client.
// If flush is true, the sink will be instructed to flush data immediately
func AdhocTime(name string, d time.Duration, flush bool) {
	defaultClient.AdhocTime(name, d, flush)
}

// AdhocSample creates an ad-hoc histogram metric event at the default client.
// If flush is true, the sink will be instructed to flush data immediately
func AdhocSample(name string, val int64, flush bool) {
	defaultClient.AdhocSample(name, val, flush)
}

// Mark - send a ad-hoc zero histogram event at the default client. - see Client.Mark()
func Mark(name string) {
	defaultClient.Mark(name)
}
