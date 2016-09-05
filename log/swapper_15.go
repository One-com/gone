// +build go1.5

package log

import (
	"errors"
	"sync"
	"sync/atomic"
)

// Indirection of Log() calls through a Handler which can be atomically swapped

// ErrNotLogged is returned by some Log functions (Log()/Output()) if no Handler was found to log an event.
var ErrNotLogged = errors.New("No handler found to log event")

// Using atomic and mutex to support atomic reads, but also read-modify-write ops.
type swapper struct {
	mu  sync.Mutex // Locked by any who want to modify the valueStruct
	val atomic.Value
}

type valueStruct struct {
	// The actual Log event Handler
	Handler
	// Any parent in the named hierarchy
	parent *Logger
}

// makes sure to initialize a swapper with a value
func newSwapper() (s *swapper) {
	s = new(swapper)
	s.val.Store(valueStruct{})
	return
}

// Log sends the event down the first Handler chain, it finds in the Logger tree.
// NB: This is different from pythong "logging" in that only one Handler is activated
func (h *swapper) Log(e *event) (err error) {

	// try the local handler
	v, _ := h.val.Load().(valueStruct)
	// Logger swappers *has* to have a valid valueStruct

	if v.Handler != nil {
		err = v.Handler.Log(Event{e})
		if err == nil {
			freePoolEvent(e)
			return
		}
	}
	// Either no handler, or an error was returned
	// Have to try parents. Walk the name-tree to find the first handler, not returning an error
	cur := v.parent.h
	for cur != nil {
		v, _ := cur.val.Load().(valueStruct) // must be valid
		if v.Handler != nil {
			err = v.Handler.Log(Event{e})
			if err == nil {
				freePoolEvent(e)
				return
			}
		}
		cur = v.parent.h
	}

	freePoolEvent(e)
	if err == nil {
		err = ErrNotLogged
	}
	return
}

func (s *swapper) SwapParent(new *Logger) (old *Logger) {
	s.mu.Lock()
	v := s.val.Load().(valueStruct)
	old = v.parent
	h := v.Handler
	s.val.Store(valueStruct{Handler: h, parent: new})
	s.mu.Unlock()
	return
}

func (s *swapper) SwapHandler(new Handler) {
	s.mu.Lock()
	p := (s.val.Load().(valueStruct)).parent
	s.val.Store(valueStruct{Handler: new, parent: p})
	s.mu.Unlock()
}

func (s *swapper) handler() Handler {
	return (s.val.Load().(valueStruct)).Handler
}

func (s *swapper) parent() *Logger {
	return (s.val.Load().(valueStruct)).parent
}
