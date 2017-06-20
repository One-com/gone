package metric

import (
	"github.com/One-com/gone/metric/num64"
	"math"
	"sync/atomic"
)

// A client maintained gauge which is only sampled regularly without information loss
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
// to implement FlushReading() fast (saving an interface allocation)
type GaugeFloat64 struct {
	name string
	val  uint64
}

// NewGauge is alias for NewGaugeUint64
func NewGauge(name string) *GaugeUint64 {
	return NewGaugeUint64(name)
}

// NewGaugeUint64 returns a standard gauge.
func NewGaugeUint64(name string) *GaugeUint64 {
	g := &GaugeUint64{name: name}
	return g
}

// FlushReading to implement Meter interface
func (g *GaugeUint64) FlushReading(s Sink) {
	val := atomic.LoadUint64(&g.val)
	n := num64.FromUint64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}

// Name to implement Meter interface
func (g *GaugeUint64) Name() string {
	return g.name
}

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

// NewGaugeInt64 creates a int64 Gauge. Can be used as a go-metric client side gauge or counter
func NewGaugeInt64(name string) *GaugeInt64 {
	g := &GaugeInt64{name: name}
	return g
}

// FlushReading sends the gauge value to the sink
func (g *GaugeInt64) FlushReading(s Sink) {
	val := atomic.LoadInt64(&g.val)
	n := num64.FromInt64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}

// Name returns the name of the gauge.
func (g *GaugeInt64) Name() string {
	return g.name
}

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
func NewGaugeFloat64(name string) *GaugeFloat64 {
	g := &GaugeFloat64{name: name}
	return g
}

// Name returns the name of the gauge
func (g *GaugeFloat64) Name() string {
	return g.name
}

// Set updates the gauge's value.
func (g *GaugeFloat64) Set(v float64) {
	atomic.StoreUint64(&g.val, math.Float64bits(v))

}

// Value returns the gauge's current value.
func (g *GaugeFloat64) Value() float64 {
	return math.Float64frombits(atomic.LoadUint64(&g.val))
}

// FlushReading sends the current gauge value to the Sink
func (g *GaugeFloat64) FlushReading(s Sink) {
	val := atomic.LoadUint64(&g.val)
	n := num64.Float64FromUint64(val)
	s.RecordNumeric64(MeterGauge, g.name, n)
}
