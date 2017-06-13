package metric

import (
	"github.com/One-com/gone/metric/num64"
	"math"
	"sync/atomic"
)

// A client maintained gauge which is only sampled regulary without information loss
// wrt. the absolute value.
// Can be used as a client side maintained counter too.

// GaugeUint64 is the default gauge type using a uint64
type GaugeUint64 struct {
	name string
	val  uint64
}

// GaugeInt64 is a gauge using an int64 - meaning it can be decremented to negaive values
type GaugeInt64 struct {
	name string
	val  int64
}

// GaugeFloat64 is a float64 gauge which stores its value as a uint64
// to implement Flush() fast
type GaugeFloat64 struct {
	name string
	val  uint64
}

// NewGauge is alias for NewGaugeUint64
func NewGauge(name string, opts ...MOption) *GaugeUint64 {
	return defaultClient.NewGauge(name, opts...)
}

// NewGauge is alias for NewGaugeUint64
func (c *Client) NewGauge(name string, opts ...MOption) *GaugeUint64 {
	return c.NewGaugeUint64(name, opts...)
}

// NewGaugeUint64 returns a standard gauge.
func (c *Client) NewGaugeUint64(name string, opts ...MOption) *GaugeUint64 {
	g := &GaugeUint64{name: name}
	c.register(g, opts...)
	return g
}

// Flush to implement Meter interface
func (g *GaugeUint64) Flush(s Sink) {
	val := atomic.LoadUint64(&g.val)
	n := num64.FromUint64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}

// Name to implement Meter interface
func (g *GaugeUint64) Name() string {
	return g.name
}

//// MeterType to implement Meter interface
//func (g *GaugeUint64) MeterType() int {
//	return MeterGauge
//}

// Set will update the gauge value
func (g *GaugeUint64) Set(val uint64) {
	atomic.StoreUint64(&g.val, val)
}

// Value returns the gauge value
func (g *GaugeUint64) Value() uint64 {
	return atomic.LoadUint64(&g.val)
}

// Inc will increment the gauge value
func (g *GaugeUint64) Inc(i uint64) {
	atomic.AddUint64(&g.val, i)
}

// Dec will decrement the gauge value
func (g *GaugeUint64) Dec(i int64) {
	atomic.AddUint64(&g.val, ^uint64(i-1))
}

// NewgaugeInt64 creates a int64 Gauge. Can be used as a go-metric client side gauge or counter
func (c *Client) NewGaugeInt64(name string, opts ...MOption) *GaugeInt64 {
	g := &GaugeInt64{name: name}
	c.register(g, opts...)
	return g
}

// Flush sends the gauge value to the sink
func (g *GaugeInt64) Flush(s Sink) {
	val := atomic.LoadInt64(&g.val)
	n := num64.FromInt64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}

// Name returns the name of the gauge.
func (g *GaugeInt64) Name() string {
	return g.name
}

//func (g *GaugeInt64) MeterType() int {
//	return MeterGauge
//}

// Set sets the gauge value
func (g *GaugeInt64) Set(val int64) {
	atomic.StoreInt64(&g.val, val)
}

// Value returns the current gauge value
func (g *GaugeInt64) Value() int64 {
	return atomic.LoadInt64(&g.val)
}

// Dec decrements the counter by the given amount.
func (g *GaugeInt64) Dec(i int64) {
	atomic.AddInt64(&g.val, -i)
}

// Inc increments the counter by the given amount.
func (g *GaugeInt64) Inc(i int64) {
	atomic.AddInt64(&g.val, i)
}

// NewGaugeFloat64 creates a gauge holding a Float64 value.
func (c *Client) NewGaugeFloat64(name string, opts ...MOption) *GaugeFloat64 {
	g := &GaugeFloat64{name: name}
	c.register(g, opts...)
	return g
}

// Name returns the name of the gauge
func (g *GaugeFloat64) Name() string {
	return g.name
}

//func (g *GaugeFloat64) MeterType() int {
//	return MeterGauge
//}

// Update updates the gauge's value.
func (g *GaugeFloat64) Set(v float64) {
	atomic.StoreUint64(&g.val, math.Float64bits(v))

}

// Value returns the gauge's current value.
func (g *GaugeFloat64) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&g.val))
}

func (g *GaugeFloat64) Flush(s Sink) {
	val := atomic.LoadUint64(&g.val)
	n := num64.Float64FromUint64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}
