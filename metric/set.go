package metric

import (
	"sync"
)

// Set is a not too efficient implementation of that statsd "set" concept.
// The client will send set membership notifications. Statsd will periodically
// further propagate the current set size as a gauge.
// It is client side buffered for the use cases where there will be a lot of
// membership Add events for the same Set member.
// This is often not the case, so the AdHocSetMember is often a simpler solution.
type Set struct {
	name string

	mu  sync.Mutex
	set map[string]struct{}
}

// NewSet creates a new named Set object
func NewSet(name string) *Set {
	s := &Set{name: name}
	s.set = make(map[string]struct{})
	return s
}

// Flush sends the current set members to the Sink and resets the Set to empty.
func (s *Set) FlushReading(f Sink) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, _ := range s.set {
		f.Record(MeterSet, s.name, k)
	}
	s.set = make(map[string]struct{})
}

// Name returns the name of the Set
func (s *Set) Name() string {
	return s.name
}

// Add a member to the set.
func (s *Set) Add(val string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.set[val] = struct{}{}
}
