package log

import (
	"github.com/One-com/gone/log/syslog"
	"io"
)

// LevelLogger makes available methods compatible with the stdlib logger and
// an extended API for leveled logging.
// LevelLogger is implemented by *log.Logger
type LevelLogger interface {

	// Will generate a log event with this level if the Logger log level is
	// high enough.
	// The event will have the given log message and key/value structured data.
	Log(level syslog.Priority, message string, kv ...interface{}) error

	// further interfaces
	StdLogger
}

// StdLogger is the interface used by the standard lib *log.Logger
// This is the API for actually logging stuff.
type StdLogger interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})

	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})

	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

// LogFunc is the type of the function returned by *ok() methods, which will
// log at the level queried about if called.
type LogFunc func(msg string, kv ...interface{})

//---

// StdFormatter allows quering a formatting handler for flags and prefix compatible with the
// stdlib log library
type StdFormatter interface {
	Flags() int
	Prefix() string
}

// StdMutableFormatter is the interface for a Logger which directly
// can change the stdlib flags, prefix and output io.Writer attributes in a synchronized manner.
// Since gonelog Handlers are immutable, it's not used for Formatters.
type StdMutableFormatter interface {
	StdFormatter
	SetFlags(flag int)
	SetPrefix(prefix string)
	SetOutput(w io.Writer)
}

// StdLoggerFull is mostly for documentation purposes. This is the full set of methods
// supported by the standard logger. You would only use the extra methods when you know
// exactly which kind of logger you are dealing with anyway.
type StdLoggerFull interface {
	StdLogger
	StdMutableFormatter
	Output(calldepth int, s string) error
}
