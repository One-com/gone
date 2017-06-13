package daemon

import (
	"sync"
)

// implements a simple log interface for syslog leveled logging.

// Syslog priority levels
const (
	LvlEMERG int = iota // Not to be used by applications.
	LvlALERT
	LvlCRIT
	LvlERROR
	LvlWARN
	LvlNOTICE
	LvlINFO
	LvlDEBUG
)

// A LoggerFunc can be set to make the daemon internal events log to a custom log library
type LoggerFunc func(level int, message string)

var (
	logmu  sync.RWMutex
	logger LoggerFunc
)

// SetLogger sets a custom log function.
func SetLogger(f LoggerFunc) {
	logmu.Lock()
	defer logmu.Unlock()
	logger = f
}

// Log is used to log internal events if a LoggerFunc is set with SetLogger()
// You can call this your self if you need to. It's go-routine safe if the provided Log function is.
// However, it's not fast. Don't use this for logging not related to daemon.Run()
func Log(level int, msg string) {
	logmu.RLock()
	if logger != nil {
		logger(level, msg)
	}
	logmu.RUnlock()
}
