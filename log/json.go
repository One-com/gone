package log

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type jsonformatter struct {
	out        io.Writer
	timelayout string
	keynames   *EventKeyNames // Names for basic event fields
}

// Clone returns a clone of the current handler for tweaking and swapping in
func (f *jsonformatter) Clone(options ...HandlerOption) CloneableHandler {
	new := &jsonformatter{}
	// We shamelessly copy the whole formatter. This is ok, since everything
	// mutable is pointer types (like pool) and we can inherit those.
	*new = *f

	for _, option := range options {
		option(new)
	}

	return new
}

// JSON Formatter Option to set flags
func KeyNamesOpt(keys *EventKeyNames) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*jsonformatter); ok {
			h.keynames = keys
		}
	}
}

// JSON Formatter Option to set timestamp formatting
func TimeFormatOpt(layout string) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*jsonformatter); ok {
			h.timelayout = layout
		}
	}
}

// NewJSONFormatter creates a new formatting Handler writing log events as JSON to the supplied Writer.
func NewJSONFormatter(w io.Writer, options ...HandlerOption) *jsonformatter {
	f := &jsonformatter{keynames: defaultKeyNames, out: w}
	// Apply the options
	for _, option := range options {
		option(f)
	}

	return f
}

func (l *jsonformatter) Log(e Event) error {
	x := len(e.Data)
	n := x/2 + 3
	m := make(map[string]interface{}, n)
	m[l.keynames.Lvl] = e.Lvl
	m[l.keynames.Msg] = e.Msg
	if l.keynames.Name != "" {
		m[l.keynames.Name] = e.Name
	}
	m[l.keynames.Time] = e.Time().Format(l.timelayout)
	for i := 0; i < x; i += 2 {
		k := e.Data[i]
		var v interface{} = errors.New("MISSING")
		if i+1 < len(e.Data) {
			v = e.Data[i+1]
		}
		merge(m, k, v)
	}
	return json.NewEncoder(l.out).Encode(m)
}

func merge(dst map[string]interface{}, k, v interface{}) {
	var key string
	switch x := k.(type) {
	case string:
		key = x
	case fmt.Stringer:
		key = safeString(x)
	default:
		key = fmt.Sprint(x)
	}
	if x, ok := v.(error); ok {
		v = safeError(x)
	}
	dst[key] = v
}

func safeString(str fmt.Stringer) (s string) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(str); v.Kind() == reflect.Ptr && v.IsNil() {
				s = "NULL"
			} else {
				panic(panicVal)
			}
		}
	}()
	s = str.String()
	return
}

func safeError(err error) (s interface{}) {
	defer func() {
		if panicVal := recover(); panicVal != nil {
			if v := reflect.ValueOf(err); v.Kind() == reflect.Ptr && v.IsNil() {
				s = nil
			} else {
				panic(panicVal)
			}
		}
	}()
	s = err.Error()
	return
}
