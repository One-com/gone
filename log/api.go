package log

import (
	"github.com/One-com/gone/log/syslog"
)

// Log is the simplest Logger method
func (l *Logger) Log(level syslog.Priority, msg string, kv ...interface{}) (err error) {
	if l.Does(level) {
		err = l.log(level, msg, kv...)
	}
	return
}

// LogFromCaller is like Log() but lets you offset the calldepth used to compute CodeInfo (file/line) from stack info, the same way as the stdlib log.Output() function does.
func (l *Logger) LogFromCaller(calldepth int, level syslog.Priority, msg string, kv ...interface{}) error {
	return l.logFromCaller(calldepth, level, msg, kv...)
}

//---

// Internal level loggers (to return from *ok() functions)
// These are functionally equivalent to log() and output(), but bound to a specific level.
func (l *Logger) alert(msg string, kv ...interface{}) {
	level := syslog.LOG_ALERT
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) crit(msg string, kv ...interface{}) {
	level := syslog.LOG_CRIT
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) error(msg string, kv ...interface{}) {
	level := syslog.LOG_ERROR
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) warn(msg string, kv ...interface{}) {
	level := syslog.LOG_WARN
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) notice(msg string, kv ...interface{}) {
	level := syslog.LOG_NOTICE
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) info(msg string, kv ...interface{}) {
	level := syslog.LOG_INFO
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}
func (l *Logger) debug(msg string, kv ...interface{}) {
	level := syslog.LOG_DEBUG
	l.h.Log(l.newEvent(1, level, msg, normalize(kv)))
}

//---

// ALERTok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) ALERTok() (LogFunc, bool) { return l.alert, l.Does(syslog.LOG_ALERT) }

// CRITok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) CRITok() (LogFunc, bool) { return l.crit, l.Does(syslog.LOG_CRIT) }

// ERRORok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) ERRORok() (LogFunc, bool) { return l.error, l.Does(syslog.LOG_ERROR) }

// WARNok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) WARNok() (LogFunc, bool) { return l.warn, l.Does(syslog.LOG_WARN) }

// NOTICEok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) NOTICEok() (LogFunc, bool) { return l.notice, l.Does(syslog.LOG_NOTICE) }

// INFOok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) INFOok() (LogFunc, bool) { return l.info, l.Does(syslog.LOG_INFO) }

// DEBUGok will return whether the logger generates events at this level, and a function which will do the logging when called.
func (l *Logger) DEBUGok() (LogFunc, bool) { return l.debug, l.Does(syslog.LOG_DEBUG) }

//---

// ALERT - Log a message and optional KV values at syslog ALERT level.
func (l *Logger) ALERT(msg string, kv ...interface{}) {
	lvl := syslog.LOG_ALERT
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// CRIT - Log a message and optional KV values at syslog CRIT level.
func (l *Logger) CRIT(msg string, kv ...interface{}) {
	lvl := syslog.LOG_CRIT
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// ERROR - Log a message and optional KV values at syslog ERROR level.
func (l *Logger) ERROR(msg string, kv ...interface{}) {
	lvl := syslog.LOG_ERROR
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// WARN - Log a message and optional KV values at syslog WARN level.
func (l *Logger) WARN(msg string, kv ...interface{}) {
	lvl := syslog.LOG_WARN
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// NOTICE - Log a message and optional KV values at syslog NOTICE level.
func (l *Logger) NOTICE(msg string, kv ...interface{}) {
	lvl := syslog.LOG_NOTICE
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// INFO - Log a message and optional KV values at syslog INFO level.
func (l *Logger) INFO(msg string, kv ...interface{}) {
	lvl := syslog.LOG_INFO
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}

// DEBUG - Log a message and optional KV values at syslog DEBUG level.
func (l *Logger) DEBUG(msg string, kv ...interface{}) {
	lvl := syslog.LOG_DEBUG
	if l.Does(lvl) {
		l.log(lvl, msg, kv...)
	}
}
