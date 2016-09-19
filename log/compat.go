package log

import (
	"fmt"
	"github.com/One-com/gone/log/syslog"
	"io"
	"os"
)

// New will instantiate a logger with the same functionality (and limitations) as the std lib logger.
func New(out io.Writer, prefix string, flags int) *Logger {
	h := NewStdFormatter(SyncWriter(out), prefix, flags)
	l := NewLogger(LvlDEFAULT, h) // not a part of the hierarchy
	l.DoTime(true)
	return l
}

// Output() compatible with the standard lib logger
func (l *Logger) Output(calldepth int, s string) error {
	return l.output(calldepth, s)
}

// Fatal() compatible with the standard lib logger
func (l *Logger) Fatal(v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprint(v...)
	l.log(lvl, s)
	os.Exit(1)
}

// Fatalf() compatible with the standard lib logger
func (l *Logger) Fatalf(format string, v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprintf(format, v...)
	l.log(lvl, s)
	os.Exit(1)
}

// Fatalln() compatible with the standard lib logger
func (l *Logger) Fatalln(v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprint(v...)
	l.log(lvl, s)
	os.Exit(1)
}

// Panic() compatible with the standard lib logger
func (l *Logger) Panic(v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprint(v...)
	l.log(lvl, s)
	panic(s)
}

// Panicf() compatible with the standard lib logger
func (l *Logger) Panicf(format string, v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprintf(format, v...)
	l.log(lvl, s)
	panic(s)
}

// Panicln compatible with the standard lib logger
func (l *Logger) Panicln(v ...interface{}) {
	lvl := syslog.LOG_ALERT
	s := fmt.Sprint(v...)
	l.log(lvl, s)
	panic(s)
}

// Print() compatible with the standard lib logger
func (l *Logger) Print(v ...interface{}) {
	if lvl, ok := l.DoingDefaultLevel(); ok {
		s := fmt.Sprint(v...)
		l.log(lvl, s)
	}
}

// Printf() compatible with the standard lib logger
func (l *Logger) Printf(format string, v ...interface{}) {
	if lvl, ok := l.DoingDefaultLevel(); ok {
		s := fmt.Sprintf(format, v...)
		l.log(lvl, s)
	}
}

// Println() compatible with the standard lib logger
func (l *Logger) Println(v ...interface{}) {
	if lvl, ok := l.DoingDefaultLevel(); ok {
		s := fmt.Sprint(v...)
		l.log(lvl, s)
	}
}

//---

// These functions have been delegated to the swapper, since some of them might
// need to replace the handler.
// If these functions have no meaning for the actual Handler attached, then they
// result in a NOOP.

// Flags() compatible with the standard lib logger
func (l *Logger) Flags() int {
	return l.h.Flags()
}

// Prefix() compatible with the standard lib logger
func (l *Logger) Prefix() string {
	return l.h.Prefix()
}

// SetFlags() compatible with the standard lib logger
func (l *Logger) SetFlags(flag int) {
	// First activate needed book keeping
	if flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		l.DoTime(true)
	}
	if flag&(Llongfile|Lshortfile) != 0 {
		l.DoCodeInfo(true)
	}

	l.h.SetFlags(flag)

	// De-activate unneeded book keeping
	if flag&(Ldate|Ltime|Lmicroseconds) == 0 {
		l.DoTime(false)
	}
	if flag&(Llongfile|Lshortfile) == 0 {
		l.DoCodeInfo(false)
	}
}

// SetPrefix() compatible with the standard lib logger
func (l *Logger) SetPrefix(prefix string) {
	l.h.SetPrefix(prefix)
}

// SetOutput() compatible with the standard lib logger
func (l *Logger) SetOutput(w io.Writer) {
	l.h.SetOutput(w)
}
