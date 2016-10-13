package metric

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

func (n Numeric64) Uint64() uint64 {
	switch n.Type {
	case Uint64:
		return n.value
	default:
		panic("Numeric64 is not an Uint64")
	}
}

func (n Numeric64) Int64() int64 {
	switch n.Type {
	case Int64:
		return int64(n.value)
	default:
		panic("Numeric64 is not an Int64")
	}
}

func (n Numeric64) Float64() float64 {
	switch n.Type {
	case Float64:
		return math.Float64frombits(n.value)
	default:
		panic("Numeric64 is not an Float64")
	}
}
