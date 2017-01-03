package log

// KV is a map of key/value pairs to pass to a Logger context or to a log function for
// structured logging.
// Value can be any stringable object, or a Valuer which resolves to a stringable object.
type KV map[string]interface{}

// KeyValues is a slice of interfaces alternating between key and value for data
// to be logged.
// Keys are at even numbered indexes and must be string.
// Values can be any stringable object or a Valuer which resolves to a stringable object.
type KeyValues []interface{}

// Logable is the interface needed for an object to be provided as a value to
// a nil key and then being allowed to generate it's k/v log values it self
// by returning them from LogValues()
type Logable interface {
	LogValues() KeyValues
}

const errorKey = "LOG_ERROR"

// Take a vararg list of arguments and make it a slice if it isn't already.
func normalize(kv []interface{}) []interface{} {
	if kv == nil {
		return nil
	}

	// if the caller passed a KV object or a Logable, then expand it
	// and insist that the rest of the arguments are also Logables
	var expkv []interface{}
	var i int
	for len(kv) > i {
		if logable, ok := kv[i].(Logable); ok {
			expkv = append(expkv, logable.LogValues()...)
		} else {
			break
		}
		i++
	}
	// Append other normal key/value pairs
	if i > 0 {
		kv = append(expkv, kv[i:]...)
	}

	// kv needs to be even because it's a series of key/value pairs
	// no one wants to check for errors on logging functions,
	// so instead of erroring on bad input, we'll just make sure
	// that things are the right length and users can fix bugs
	// when they see the output looks wrong
	if len(kv)%2 != 0 {
		kv = append(kv, nil, errorKey, "Normalized odd number of arguments by adding nil")
	}

	return kv
}

// KV is a Logable
func (kv KV) LogValues() KeyValues {
	arr := make([]interface{}, len(kv)*2)

	i := 0
	for k, v := range kv {
		arr[i] = k
		arr[i+1] = v
		i += 2
	}

	return arr
}
