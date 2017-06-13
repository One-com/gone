package num64

import (
	"math"
)

// Data types for Numeric64
const (
	Uint64 = iota
	Int64
	Float64
)

// Numeric64 is an internal datatype used to generalize an eventStream to hold
// any 64 bit numeric value. This is a performance optimation. An interface{} would
// require additional dynamic allocation of heap objects.
type Numeric64 struct {
	Type  int
	value uint64
}

func FromUint64(v uint64 ) Numeric64 {
	return Numeric64{Type: Uint64, value: v}
}

func FromInt64(v int64) Numeric64 {
	return Numeric64{Type: Int64, value: uint64(v)}
}

func FromFloat64(v float64) Numeric64 {
	return Numeric64{Type: Float64, value: math.Float64bits(v)}
}

func Float64FromUint64(v uint64) Numeric64 {
	return Numeric64{Type: Float64, value: v}
}


// Uint64 returns the 64-bit values as a uint64
func (n Numeric64) Uint64() uint64 {
	switch n.Type {
	case Uint64:
		return n.value
	default:
		panic("Numeric64 is not an Uint64")
	}
}

// Int64 returns the 64-bit values as an int64
func (n Numeric64) Int64() int64 {
	switch n.Type {
	case Int64:
		return int64(n.value)
	default:
		panic("Numeric64 is not an Int64")
	}
}

// Float64 returns the 64-bit value as a float64
func (n Numeric64) Float64() float64 {
	switch n.Type {
	case Float64:
		return math.Float64frombits(n.value)
	default:
		panic("Numeric64 is not an Float64")
	}
}
