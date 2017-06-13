package metric

import (
	"github.com/One-com/gone/metric/num64"
	"sync/atomic"
)

// Counter is different from a GaugeInt64 in that it is reset to zero every
// time its flushed - and thus being server-side maintained.
type Counter struct {
	name string
	val  int64
}

// A server side maintained counter. "Server side" meaning that it's
// reset to 0 every time it's sent to the server and the tally is kept on the server
// This poses the risk of the server-side absolute value to drift in case of increments
// lost in transit. However, it also allows several processes to update the same counter.
// If you want to have a client side counter, use GaugeInt64
func NewCounter(name string, opts ...MOption) *Counter {
	return defaultClient.NewCounter(name, opts...)
}

func (c *Client) NewCounter(name string, opts ...MOption) *Counter {
	g := &Counter{name: name}
	c.register(g, opts...)
	return g
}

func (c *Counter) Flush(s Sink) {
	val := atomic.SwapInt64(&c.val, 0)
	if val != 0 {
		n := num64.FromInt64(int64(val))
		s.RecordNumeric64(MeterCounter, c.name, n)
	}
}

// Name returns the name of the counter
func (c *Counter) Name() string {
	return c.name
}

//func (c *Counter) MeterType() int {
//	return MeterCounter
//}

func (c *Counter) Inc(val int64) {
	atomic.AddInt64(&c.val, val)
}

func (c *Counter) Dec(i int64) {
	atomic.AddInt64(&c.val, -i)
}
