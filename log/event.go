package log

import (
	"github.com/One-com/gone/log/syslog"
	"runtime"
	"sync"
	"time"
)

// Event is the basic log event type.
// Exported to be able to implement Handler interface for external packages.
// Handlers passed an Event "e" can access e.Lvl, e.Msg, e.Data, e.Name
type Event struct {
	*event
}

var evpool *sync.Pool

func init() {
	evpool = &sync.Pool{New: func() interface{} { return new(event) }}
}

func getPoolEvent(l syslog.Priority, name string, msg string) *event {
	e := evpool.Get().(*event)
	*e = event{}
	e.Lvl = l
	e.Msg = msg
	e.Name = name
	return e
}
func freePoolEvent(e *event) {
	evpool.Put(e)
}

// A log event.
// Not exported to discourage manual instantiation.
// Exported fields to allow external Handlers (and formatting Handlers)
// Do *not* instantiate these yourself. They are meant to be immutable once created.
// There's no method to pass a home-made Event to a Logger and a pandoras box of
// race conditions open if there were.
// Also, the event is put back into the pool once logged. It has to come from the pool
type event struct {
	Lvl  syslog.Priority // Level this event was logged at.
	Msg  string          // Basic log message.
	Data []interface{}   // Structured data unique for this event
	Name string          // Name of the logger generating this event.

	// Time is only evaluated if needed
	tok  bool
	time time.Time

	// So is file/line/stack information
	fok  bool
	file string
	line int
}

// EventKeyNames holds keynames for fixed event fields, when needed (such as in JSON)
type EventKeyNames struct {
	Lvl  string
	Name string
	Time string
	Msg  string
	File string
	Line string
}

var defaultKeyNames = &EventKeyNames{
	Lvl:  "_lvl",
	Name: "_name",
	Time: "_ts",
	Msg:  "_msg",
	File: "_file",
	Line: "_line",
}

// Time returns the timestamp of an event.
func (e *event) Time() (t time.Time) {
	if e.tok {
		return e.time
	}

	// don't modify an event after creation
	// If you need a single timestamp for all usages of the event,
	// enable DoTime() in the Logger
	return time.Now() // get some timestamp
}

// FileInfo returns the file and line number of a log event.
func (e Event) FileInfo() (string, int) {
	return e.file, e.line
}

// To support stdlib Output() function which gives user control of calldepth.
func (l *Logger) calldepthEvent(level syslog.Priority, calldepth int, msg string) *event {

	e := getPoolEvent(level, l.name, msg)

	dt, dc := l.cfg.doing()
	if dt {
		e.time = time.Now()
		e.tok = true
	}
	if dc {
		_, file, line, ok := runtime.Caller(calldepth + 2)
		if ok {
			e.file = file
			e.line = line
			e.fok = true
		}
	}

	if l.cparent == nil && l.data != nil {
		// Traverse contexts gather KV data
		var i int
		// tally up the kv length
		parent := l
		for parent != nil {
			i += len(parent.data)
			parent = parent.cparent
		}
		newdata := make([]interface{}, i)
		// Now collect data
		parent = l
		i = 0
		for parent != nil {
			for _, k_or_v := range parent.data {
				newdata[i] = k_or_v
				i++
			}
			parent = parent.cparent
		}
		e.Data = newdata
	}
	return e
}

// The primary event constructor. Creates a new log event.
// The event is timestamped if needed
// Code info file/line is recorded if needed
// KV data is gathered from any context parents.
func (l *Logger) newEvent(level syslog.Priority, msg string, data []interface{}) *event {

	e := getPoolEvent(level, l.name, msg)

	dt, dc := l.cfg.doing()
	if dt {
		e.time = time.Now()
		e.tok = true
	}
	if dc {
		_, file, line, ok := runtime.Caller(3)
		if ok {
			e.file = file
			e.line = line
			e.fok = true
		}
	}

	if l.cparent == nil && l.data == nil {
		e.Data = data
	} else {
		// Traverse contexts gather KV data
		var i int
		// tally up the kv length
		parent := l
		for parent != nil {
			i += len(parent.data)
			parent = parent.cparent
		}
		newdata := make([]interface{}, i+len(data))
		// Now collect data
		parent = l
		i = 0
		for parent != nil {
			for _, k_or_v := range parent.data {
				newdata[i] = k_or_v
				i++
			}
			parent = parent.cparent
		}
		// add the event specific kv data
		for _, k_or_v := range data {
			newdata[i] = k_or_v
			i++
		}
		e.Data = newdata
	}
	return e
}
