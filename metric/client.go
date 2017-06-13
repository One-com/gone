package metric

import (
	"sync"
	"time"
	"github.com/One-com/gone/metric/num64"
)

// Client is the main object the applications holds to do metrics.
// It can be used directly for ad-hoc events, or be used to create persistent
// gauge/conter/timer/... objects optimised for bulk metric generation.
// A default metric Client with no FlushInterval set is Start()'ed by init()
type Client struct {

	// Wait for all flushers to empty before exiting Stop()
	done *sync.WaitGroup

	// protects the flusher map and the running flag.
	fmu      sync.Mutex
	// a map of flushers doing flushing at given intervals
	// - to reuse a flusher for metrics with same interval
	flushers map[time.Duration]*flusher

	// The flusher handling meters which have been given no specific
	// flush interval
	defaultFlusher *flusher

	// Factory for generating new independent sink objects for flushers.
	sinkf SinkFactory

	running bool
}

// A single global client. Remember to set a default sink
var defaultClient *Client

func init() {
	defaultClient = NewClient(nil)
	defaultClient.Start()
}

// NewClient returns you a client handle directly if you do not want to use the
// global default client.
// This creates a new metric client with a factory object for the sink.
// If sink == nil, the client will not emit metrics until a Sink is set using
// SetSink.
func NewClient(sinkf SinkFactory, opts ...MOption) (client *Client) {

	client = &Client{}
	client.done = new(sync.WaitGroup)
	client.flushers = make(map[time.Duration]*flusher)

	// create a default flusher which do not flush by it self.
	client.defaultFlusher = newFlusher(0)

	if sinkf != nil {
		client.SetSink(sinkf)
	}
	
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
func SetDefaultSink(sinkf SinkFactory) {
	c := defaultClient
	c.SetSink(sinkf)
}

// SetSink sets the Sink factory of the client.
// You'll need to set a sink before any metrics will be emitted.
func (c *Client) SetSink(sinkf SinkFactory) {
	c.fmu.Lock()

	c.sinkf = sinkf
	c.defaultFlusher.setSink(sinkf)
	for _, f := range c.flushers {
		if sinkf == nil {
			f.setSink(&nilSinkFactory{})
		} else {
			f.setSink(sinkf)
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
// If any AutoFlusher meters are still in use they will still flush when overflown.
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

// register a Meter with the client, finding a flusher
// with the appropriate interval, if possible, else create a new flusher.
func (c *Client) register(m meter, opts ...MOption) {
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
			if c.sinkf != nil {
				f.setSink(c.sinkf)
			}
			c.flushers[flush] = f
			if c.running {
				c.done.Add(1)
				go f.run(c.done)
			}
		}
		f.register(m)
	} else {
		c.defaultFlusher.register(m)
	}
	c.fmu.Unlock()
}

//--------------------------------------------------------------

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

// Counter creates an ad-hoc counter metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) Counter(name string, val int, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterCounter, name, num64.FromInt64(int64(val)), flush)
}

// Gauge creates an ad-hoc gauge metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) Gauge(name string, val uint64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterGauge, name, num64.FromUint64(val), flush)
}

// Timer creates an ad-hoc timer metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) Timer(name string, d time.Duration, flush bool) {
	val := d.Nanoseconds() / int64(1000000)
	c.defaultFlusher.RecordNumeric64(MeterTimer, name, num64.FromInt64(int64(val)), flush)
}

// Sample creates an ad-hoc histogram metric event.
// If flush is true, the sink will be instructed to flush data immediately
func (c *Client) Sample(name string, val int64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, num64.FromInt64(int64(val)), flush)
}

// Mark - send a zero histogram event immediately to allow the server side to indicate a unique event happened. This equivalent to calling Sample(name, 0, true) and can be used as a poor mans way to make qualitative events to be marked in the overall view of metrics. Like "process restart". Graphical views might allow you to draw these as special marks. For some sinks (like statsd) there's not dedicated way to send such events.
func (c *Client) Mark(name string) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, num64.FromInt64(int64(0)), true)
}
