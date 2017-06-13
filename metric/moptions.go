package metric

import (
	"time"
)

// MConfig holds configuration state for metric clients and objects.
// This is an internal type.
type MConfig struct {
	cfg map[string]interface{}
}

// MOption is a function manipulating configuration state for a metrics Client or object.
type MOption func(MConfig)

// FlushInterval returns a configuration option for a metrics Client or object.
// Provide this to NewClient or to New{Gauge,Client,Timer..}
func FlushInterval(d time.Duration) MOption {
	return MOption(func(m MConfig) {
		m.cfg["flushInterval"] = d
	})
}
