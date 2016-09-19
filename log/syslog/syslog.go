// Package syslog provides the syslog level contants source code compatible with the standard library.
package syslog

import (
	stdsyslog "log/syslog"
)

// Priority is the std library syslog.Priority type
type Priority stdsyslog.Priority

// Stdlib syslog level constants
const (
	LOG_EMERG Priority = iota
	LOG_ALERT
	LOG_CRIT
	LOG_ERR
	LOG_WARNING
	LOG_NOTICE
	LOG_INFO
	LOG_DEBUG
)

// Aliases
const (
	LOG_ERROR Priority = LOG_ERR
	LOG_WARN  Priority = LOG_WARNING
)
