package metric

import (
	"sync"
	"time"
)

type Client struct {

	// Wait for all flushers to empty before exiting Stop()
	done *sync.WaitGroup

	// protects the flusher map
	fmu sync.Mutex
	flushers map[time.Duration]*flusher

	// The flusher handling meters which have been given no specific
	// flush interval
	defaultFlusher *flusher

	sinkf SinkFactory

	running bool
}

// A single global client. Remember to set a default sink
var defaultClient *Client

func init() {
	defaultClient = NewClient(nil)
}

// NewClient returns you a client handle directly if you do not want to use the
// global default client.
// Create a new metric client with a factory object for the sink.
// If sink == nil, the client will not emit metrics until a Sink is set.
func NewClient(sinkf SinkFactory, opts ...MOption) (client *Client) {

	client = &Client{}
	client.done =  new(sync.WaitGroup)
	client.flushers = make(map[time.Duration]*flusher)

	client.defaultFlusher = newFlusher(0)

	if sinkf != nil {
		client.SetSink(sinkf)
	}
	client.Start()
	client.SetOptions(opts...)

	return
}

// Set options on the default metric client
func SetDefaultOptions(opts ...MOption) {
	c := defaultClient
	c.SetOptions(opts...)
}

// Set options on a client - like the flush interval for metrics which
// haven't them selves a fixed flush interval
func (c *Client) SetOptions(opts ...MOption) {
	conf := make(map[string]interface{})
	for _, o := range opts {
		o(conf)
	}

	c.fmu.Lock()
	if f,ok := conf["flushInterval"]; ok {
		if d, ok := f.(time.Duration) ; ok {
			c.defaultFlusher.setInterval(d)
		}
	}
	c.fmu.Unlock()
}

func SetDefaultSink(sinkf SinkFactory) {
	c := defaultClient
	c.SetSink(sinkf)
}

// The the Sink factory of the client
func (c *Client) SetSink(sinkf SinkFactory) {
	c.fmu.Lock()

	c.sinkf = sinkf
	c.defaultFlusher.setSink(sinkf)
	for _,f := range c.flushers {
		if sinkf == nil {
			f.setSink(&nilSinkFactory{})
		} else {
			f.setSink(sinkf)
		}
	}
	c.fmu.Unlock()
}

// Start the default client if stopped.
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

	for _,f := range c.flushers {
		c.done.Add(1)
		f.run(c.done)
	}

	c.running = true
}

// Stops the global default metrics client
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

	for _,f := range c.flushers {
		f.stop()
	}
	c.done.Wait()
	c.running = false
}

func (c *Client) register(m Meter, opts ...MOption) {
	c.fmu.Lock()

	var f *flusher
	var flush time.Duration

	conf := make(map[string]interface{})
	for _, o := range opts {
		o(conf)
	}

	if fi,ok := conf["flushInterval"]; ok {
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

// Flush the client so no data is left in any pipeline buffers
func (c *Client) Flush() {
	c.defaultFlusher.Flush()
	c.defaultFlusher.FlushSink()
}

func (c *Client) Counter(name string, val int, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterCounter, name, Numeric64{Type: Int64, value: uint64(val)}, flush)
}

func (c *Client) Gauge(name string, val uint64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterGauge, name, Numeric64{Type: Uint64, value: uint64(val)}, flush)
}

func (c *Client) Timer(name string, d time.Duration, flush bool) {
	val := d.Nanoseconds()/int64(1000000)
	c.defaultFlusher.RecordNumeric64(MeterTimer, name, Numeric64{Type: Uint64, value: uint64(val)}, flush)
}

func (c *Client) Sample(name string, val int64, flush bool) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, Numeric64{Type: Int64, value: uint64(val)}, flush)
}

// Mark - send a zero event immediately to allow the server side to indicate a unique event happened.
func (c *Client) Mark(name string) {
	c.defaultFlusher.RecordNumeric64(MeterHistogram, name, Numeric64{Type: Int64, value: 0}, true)
}
