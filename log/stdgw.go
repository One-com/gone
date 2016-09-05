package log

import (
	"github.com/One-com/gone/log/syslog"
	"io"
	"log"
	"regexp"
	"strings"
)

// We can use basically the same code as the Gokit/log package:

// StdlibWriter implements io.Writer by invoking the stdlib log.Print. It's
// designed to be passed to a Formatting Handler as the Writer, for cases where
// it's necessary to redirect all gonelog log output to the stdlib logger.
type StdlibWriter struct{}

// Write implements io.Writer.
func (w StdlibWriter) Write(p []byte) (int, error) {
	log.Print(strings.TrimSpace(string(p)))
	return len(p), nil
}

//---

// StdlibAdapter wraps a Logger and allows it to be passed to the stdlib
// logger's SetOutput. It will extract date/timestamps, filenames, and
// messages, and place them under relevant keys.
// It uses regular expressions to parse the output of the standard logger to parse
// it in structured form to the gonelog logger.
// This is not an ideal solution. Prefer to use a Gonelogger directly
type StdlibAdapter struct {
	parse        bool // attempt to parse STDLIB log output to re-create event
	level        syslog.Priority
	gonelogger   *Logger
	timestampKey string
	fileKey      string
	messageKey   string
}

// StdlibAdapterOption sets a parameter for the StdlibAdapter.
type StdlibAdapterOption func(*StdlibAdapter)

// TimestampKey sets the key for the timestamp field. By default, it's "ts".
func TimestampKey(key string) StdlibAdapterOption {
	return func(a *StdlibAdapter) { a.timestampKey = key }
}

// FileKey sets the key for the file and line field. By default, it's "file".
func FileKey(key string) StdlibAdapterOption {
	return func(a *StdlibAdapter) { a.fileKey = key }
}

// MessageKey sets the key for the actual log message. By default, it's "msg".
func MessageKey(key string) StdlibAdapterOption {
	return func(a *StdlibAdapter) { a.messageKey = key }
}

// MessageKey sets the key for the actual log message. By default, it's "msg".
func Parse() StdlibAdapterOption {
	return func(a *StdlibAdapter) { a.parse = true }
}

// NewStdlibAdapter returns a new StdlibAdapter wrapper around the passed
// logger. It's designed to be passed to log.SetOutput.
func NewStdlibAdapter(logger *Logger, level syslog.Priority, options ...StdlibAdapterOption) io.Writer {
	a := StdlibAdapter{
		level:        level,
		gonelogger:   logger,
		timestampKey: "ts",
		fileKey:      "file",
		messageKey:   "msg",
	}
	for _, option := range options {
		option(&a)
	}
	return a
}

func (a StdlibAdapter) Write(p []byte) (int, error) {
	var msg string
	if a.parse {
		result := subexps(p)
		keyvals := []interface{}{}
		var timestamp string
		if date, ok := result["date"]; ok && date != "" {
			timestamp = date
		}
		if time, ok := result["time"]; ok && time != "" {
			if timestamp != "" {
				timestamp += " "
			}
			timestamp += time
		}
		if timestamp != "" {
			keyvals = append(keyvals, a.timestampKey, timestamp)
		}
		if file, ok := result["file"]; ok && file != "" {
			keyvals = append(keyvals, a.fileKey, file)
		}

		msg = result["msg"]

	} else {
		msg = string(p)
	}
	if err := a.gonelogger.log(a.level, msg); err != nil {
		return 0, err
	}
	return len(p), nil
}

const (
	logRegexpDate = `(?P<date>[0-9]{4}/[0-9]{2}/[0-9]{2})?[ ]?`
	logRegexpTime = `(?P<time>[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?)?[ ]?`
	logRegexpFile = `(?P<file>.+?:[0-9]+)?`
	logRegexpMsg  = `(: )?(?P<msg>.*)`
)

var (
	logRegexp = regexp.MustCompile(logRegexpDate + logRegexpTime + logRegexpFile + logRegexpMsg)
)

func subexps(line []byte) map[string]string {
	m := logRegexp.FindSubmatch(line)
	if len(m) < len(logRegexp.SubexpNames()) {
		return map[string]string{}
	}
	result := map[string]string{}
	for i, name := range logRegexp.SubexpNames() {
		result[name] = string(m[i])
	}
	return result
}
