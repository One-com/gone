package log

import (
	"fmt"
)

// Lazy evaluation of values to log
// returns stringable objects
type Lazy func() interface{}

// bindValues replaces all value elements (odd indexes) containing a Lazy
// with their generated value.
func bindLazy(keyvals []interface{}) {
	for i := 1; i < len(keyvals); i += 2 {
		if v, ok := keyvals[i].(Lazy); ok {
			keyvals[i] = v.evaluate()
		}
	}
}

func (l Lazy) evaluate() string {
	v := l()
	if s, ok := v.(string); ok {
		return s
	}
	if s, ok := v.(fmt.Stringer); ok {
		return s.String()
	}
	return fmt.Sprintln(v)

}
