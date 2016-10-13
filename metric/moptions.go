package metric

import (
	"time"
)

type MConfig map[string]interface{}

type MOption func(MConfig)

func FlushInterval(d time.Duration) MOption {
	return MOption(func(m MConfig) {
		m["flushInterval"] = d
	})
}
