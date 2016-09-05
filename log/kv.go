package log

// KV is a map of key/value pairs to pass to a Logger context or to a log function for
// structured logging.
// Value can be any stringable object, or a Valuer which resolves to a stringable object.
type KV map[string]interface{}

const errorKey = "LOG_ERROR"

// Take a vararg list of arguments and make it a slice if it isn't already.
func normalize(ctx []interface{}) []interface{} {
	if ctx == nil {
		return nil
	}

	// if the caller passed a KV object, then expand it
	if len(ctx) == 1 {
		if ctxMap, ok := ctx[0].(KV); ok {
			ctx = ctxMap.toArray()
		}
	}

	// ctx needs to be even because it's a series of key/value pairs
	// no one wants to check for errors on logging functions,
	// so instead of erroring on bad input, we'll just make sure
	// that things are the right length and users can fix bugs
	// when they see the output looks wrong
	if len(ctx)%2 != 0 {
		ctx = append(ctx, nil, errorKey, "Normalized odd number of arguments by adding nil")
	}

	return ctx
}

func (c KV) toArray() []interface{} {
	arr := make([]interface{}, len(c)*2)

	i := 0
	for k, v := range c {
		arr[i] = k
		arr[i+1] = v
		i += 2
	}

	return arr
}
