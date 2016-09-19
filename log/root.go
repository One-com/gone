package log

import (
	"fmt"
	"github.com/One-com/gone/log/syslog"
	"io"
	"os"
)

// All the toplevel package functionality

// The default log context
var defaultLogger *Logger

// Returns the default Logger - which is also the root of the name hierarchy.
func Default() *Logger {
	return defaultLogger
}

func init() {
	// Default Logger is an ordinary stdlib like logger, to be compatible
	defaultLogger = New(os.Stderr, "", LstdFlags)
	man = newManager(defaultLogger)
}

// Sets the default logger to the minimal mode, where it doesn't log timestamps
// But only emits systemd/syslog-compatible "<level>message" lines.
func Minimal() {
	minHandler := NewMinFormatter(SyncWriter(os.Stdout))
	defaultLogger.SetHandler(minHandler)
	// turn of doing timestamps *after* not using them
	defaultLogger.DoTime(false)
}

// Create a child K/V logger of the default logger
func With(kv ...interface{}) *Logger {
	return defaultLogger.With(kv...)
}

// AutoColoring turns on coloring if the output Writer is connected to a TTY
func AutoColoring() {
	defaultLogger.AutoColoring()
}

//--- level logger stuff

// ALERT - Requests the default logger to create a log event
func ALERT(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// CRIT - Requests the default logger to create a log event
func CRIT(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_CRIT
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// ERROR - Requests the default logger to create a log event
func ERROR(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ERROR
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// WARN - Requests the default logger to create a log event
func WARN(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_WARN
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// NOTICE - Requests the default logger to create a log event
func NOTICE(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_NOTICE
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// INFO - Requests the default logger to create a log event
func INFO(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_INFO
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// DEBUG - Requests the default logger to create a log event
func DEBUG(msg string, kv ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_DEBUG
	if c.Does(l) {
		c.log(l, msg, kv...)
	}
}

// ALERTok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func ALERTok() (LogFunc, bool) { l := defaultLogger; return l.alert, l.Does(syslog.LOG_ALERT) }

// CRITok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func CRITok() (LogFunc, bool) { l := defaultLogger; return l.crit, l.Does(syslog.LOG_CRIT) }

// ERRORok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func ERRORok() (LogFunc, bool) { l := defaultLogger; return l.error, l.Does(syslog.LOG_ERROR) }

// WARNok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func WARNok() (LogFunc, bool) { l := defaultLogger; return l.warn, l.Does(syslog.LOG_WARN) }

// NOTICEok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func NOTICEok() (LogFunc, bool) { l := defaultLogger; return l.notice, l.Does(syslog.LOG_NOTICE) }

// INFOok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func INFOok() (LogFunc, bool) { l := defaultLogger; return l.info, l.Does(syslog.LOG_INFO) }

// DEBUGok - If the default Logger is logging at the requested level a function creating such a log event will be returned.
func DEBUGok() (LogFunc, bool) { l := defaultLogger; return l.debug, l.Does(syslog.LOG_DEBUG) }

//---

// IncLevel - Increase the log level of the default Logger
func IncLevel() bool {
	return defaultLogger.IncLevel()
}

// DecLevel - Decrease the log level of the default Logger
func DecLevel() bool {
	return defaultLogger.DecLevel()
}

// SetLevel set the Logger log level.
// returns success
func SetLevel(level syslog.Priority) bool {
	return defaultLogger.SetLevel(level)
}


// SetPrintLevel sets the log level used by Print*() calls.
// If the second argument is true, Println(), Printf(), Print() will respect the Logger log level.
// If the second argument is false, log event will be generated regardless of Logger log level.
// Handlers and Writers may still filter the event out.
func SetPrintLevel(level syslog.Priority, respect bool) bool {
	return defaultLogger.SetPrintLevel(level, respect)
}

// Level returns the default Loggers log level.
func Level() syslog.Priority {
	return defaultLogger.Level()
}

//--- std logger stuff

// Output compatible with the standard library
func Output(calldepth int, s string) error {
	return defaultLogger.Output(calldepth+1, s)
}

// Flags - Compatible with the standard library
func Flags() int {
	return defaultLogger.Flags()
}

// Prefix - Compatible with the standard library
func Prefix() string {
	return defaultLogger.Prefix()
}

// SetFlags - Compatible with the standard library
func SetFlags(flag int) {
	defaultLogger.SetFlags(flag)
}

// SetPrefix - Compatible with the standard library
func SetPrefix(prefix string) {
	defaultLogger.SetPrefix(prefix)
}

// SetOutput - Compatible with the standard library
func SetOutput(w io.Writer) {
	defaultLogger.SetOutput(w)
}

// Fatal - Compatible with the standard library
func Fatal(v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprint(v...)
		c.log(l, s)
	}
	os.Exit(1)
}

// Fatalf - Compatible with the standard library
func Fatalf(format string, v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprintf(format, v...)
		c.log(l, s)
	}
	os.Exit(1)

}

// Fatalln - Compatible with the standard library
func Fatalln(v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprintln(v...)
		c.log(l, s)
	}
	os.Exit(1)
}

// Panic - Compatible with the standard library
func Panic(v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprint(v...)
		c.log(l, s)
		panic(s)
	}
}

// Panicf - Compatible with the standard library
func Panicf(format string, v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprintf(format, v...)
		c.log(l, s)
		panic(s)
	}
}

// Panicln - Compatible with the standard library
func Panicln(v ...interface{}) {
	c := defaultLogger
	l := syslog.LOG_ALERT
	if c.Does(l) {
		s := fmt.Sprintln(v...)
		c.log(l, s)
		panic(s)
	}
}

// Print - Compatible with the standard library
func Print(v ...interface{}) {
	c := defaultLogger
	if l, ok := c.DoingDefaultLevel(); ok {
		s := fmt.Sprint(v...)
		c.log(l, s)
	}
}

// Printf -  Compatible with the standard library
func Printf(format string, v ...interface{}) {
	c := defaultLogger
	if l, ok := c.DoingDefaultLevel(); ok {
		s := fmt.Sprintf(format, v...)
		c.log(l, s)
	}
}

// Println - Compatible with the standard library
func Println(v ...interface{}) {
	c := defaultLogger
	if l, ok := c.DoingDefaultLevel(); ok {
		s := fmt.Sprintln(v...)
		c.log(l, s)
	}
}

// Log is the simplest Logger method. Provide the log level (syslog.LOG_*) your self.
func Log(level syslog.Priority, msg string, kv ...interface{}) {
	c := defaultLogger
	if c.Does(level) {
		c.log(level, msg, kv...)
	}
}
