package log

import (
	"bytes"
	"fmt"
	"github.com/One-com/gone/log/syslog"
	"github.com/One-com/gone/log/term"
	"github.com/go-logfmt/logfmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// Formatters are a special kind of Handlers,
// which doesn't chain to other Handlers, but
// instead transforms the *Event to a []byte and calls an io.Writer

// dynamic formatting for the stdformatter
var (
	levelColors  = [8]string{"30", "31;1;7", "31;1", "31", "33", "32", "37", "37;2"}
	termLvlPfx   = [8]string{"[EMR]", "[ALT]", "[CRT]", "[ERR]", "[WRN]", "[NOT]", "[INF]", "[DBG]"}
	syslogLvlPfx = [8]string{"<0>", "<1>", "<2>", "<3>", "<4>", "<5>", "<6>", "<7>"}
	pid          = os.Getpid()
)

// These flags define which text to prefix to each log entry generated by the Logger.
// Extension of the std log library
const (
	// Bits or'ed together to control what's printed.
	// There is no control the format they present (as described in the comments).
	// The prefix is followed by a colon only when Llongfile or Lshortfile
	// is specified.
	// For example, flags Ldate | Ltime (or LstdFlags) produce,
	//	2009/01/23 01:23:23 message
	// while flags Ldate | Ltime | Lmicroseconds | Llongfile produce,
	//	2009/01/23 01:23:23.123123 /a/b/c/d.go:23: message
	Ldate         = 1 << iota // the date in the local time zone: 2009/01/23
	Ltime                     // the time in the local time zone: 01:23:23
	Lmicroseconds             // microsecond resolution: 01:23:23.123123.  assumes Ltime.
	Llongfile                 // full file name and line number: /a/b/c/d.go:23
	Lshortfile                // final file name element and line number: d.go:23. overrides Llongfile
	LUTC                      // if Ldate or Ltime is set, use UTC rather than the local time zone

	Llevel // prefix the log line with a syslog level <L>
	Lpid   // Include the process ID
	Lcolor // Do color logging to terminals
	Lname  // Log the name of the Logger generating the event

	LstdFlags = Ldate | Ltime // stdlib compatible

	LminFlags = Llevel // Simple systemd/syslog compatible level spec. Let external log system take care of timestamps etc.
)

// For better performance in parallel use, don't lock the whole formatter
// from start to end, but only during Write(). This requires a buffer pool.
type buffer struct {
	bytes.Buffer
	tmp [128]byte // temporary byte array for creating headers.
}

var pool sync.Pool

func init() {
	pool = sync.Pool{New: func() interface{} { return new(buffer) }}
}

func getBuffer() *buffer {
	b := pool.Get().(*buffer)
	b.Reset()
	return b
}
func putBuffer(b *buffer) {
	pool.Put(b)
}

// Basic line based formatting handler
type stdformatter struct {
	flag   int    // controlling the format
	prefix string // prefix to write at beginning of each line, after any level/timestamp
	out    io.Writer

	pfxarr *[8]string // prefixes to log lines for the 8 syslog levels.
}

// NewMinFormatter creates a standard formatter and applied the supplied options
// It will default log.LminFlags to provide simple <level>message logging
func NewMinFormatter(w io.Writer, options ...HandlerOption) *stdformatter {
	// Default formatter
	f := &stdformatter{
		flag:   LminFlags,
		pfxarr: &syslogLvlPfx,
		out:    w,
	}
	// Apply the options
	for _, option := range options {
		option(f)
	}
	return f
}

// NewStdFormatter creates a standard formatter capable of simulating the standard
// library logger.
func NewStdFormatter(w io.Writer, prefix string, flag int) *stdformatter {
	f := &stdformatter{
		out:    w,
		pfxarr: &syslogLvlPfx,
		prefix: prefix,
		flag:   flag,
	}
	return f
}

// Clone returns a clone of the current handler for tweaking and swapping in
func (f *stdformatter) Clone(options ...HandlerOption) CloneableHandler {
	new := &stdformatter{}
	// We shamelessly copy the whole formatter. This is ok, since everything
	// mutable is pointer types (like pool) and we can inherit those.
	*new = *f

	for _, option := range options {
		option(new)
	}

	return new
}

// To support stdlib query functions
func (f *stdformatter) Prefix() string {
	return f.prefix
}
func (f *stdformatter) Flags() int {
	return f.flag
}

// Generate options to create a new Handler
func (f *stdformatter) AutoColoring() HandlerOption {
	return func(c CloneableHandler) {
		var istty bool
		if o, ok := c.(*stdformatter); ok {
			w := o.out
			if tw, ok := w.(MaybeTtyWriter); ok {
				istty = tw.IsTty()
			} else {
				istty = term.IsTty(w)
			}

			if istty {
				o.flag = o.flag | Lcolor
			} else {
				o.flag = o.flag & ^Lcolor
			}
		}
	}
}

// SetFlags creates a HandlerOption to set flags.
// This is a method wrapper around FlagsOpt to be able to have the swapper
// call it genericly on different formatters to support
// stdlib operations SetFlags/SetPrefix/SetOutput
func (f *stdformatter) SetFlags(flags int) HandlerOption {
	return FlagsOpt(flags)
}

// SetPrefix creates a HandlerOption to set the formating prefix
// This is a method wrapper to be able to have the swapper
// call it genericly on different formatters to support
// stdlib operations SetFlags/SetPrefix/SetOutput
func (f *stdformatter) SetPrefix(prefix string) HandlerOption {
	return PrefixOpt(prefix)
}

// SetOutput creates a HandlerOption to se the output writer
// This is a method wrapper to be able to have the swapper
// call it genericly on different formatters to support
// stdlib operations SetFlags/SetPrefix/SetOutput
func (f *stdformatter) SetOutput(w io.Writer) HandlerOption {
	return OutputOpt(w)
}

// FlagsOpt - Standard Formatter Option to set flags
func FlagsOpt(flags int) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*stdformatter); ok {
			h.flag = flags
		}
	}
}

// PrefixOpt - Standard Formatter option to set Prefix
func PrefixOpt(prefix string) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*stdformatter); ok {
			h.prefix = prefix
		}
	}
}

// OutputOpt - Standard Formatter option so set Output
func OutputOpt(w io.Writer) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*stdformatter); ok {
			h.out = w
		}
	}
}

// LevelPrefixOpt - Standard Formatter option to set LevelPrefixes
func LevelPrefixOpt(arr *[8]string) HandlerOption {
	return func(c CloneableHandler) {
		if h, ok := c.(*stdformatter); ok {
			h.pfxarr = arr
		}
	}
}

/*********************************************************************/

// Cheap integer to fixed-width decimal ASCII.  Give a negative width to avoid zero-padding.
func itoa(buf *[]byte, i int, wid int) {
	// Assemble decimal in reverse order.
	var b [20]byte
	bp := len(b) - 1
	for i >= 10 || wid > 1 {
		wid--
		q := i / 10
		b[bp] = byte('0' + i - q*10)
		bp--
		i = q
	}
	// i < 10
	b[bp] = byte('0' + i)
	*buf = append(*buf, b[bp:]...)
}

/*********************************************************************/

func (f *stdformatter) Log(e Event) error {

	var now time.Time
	var file string
	var line int

	msg := e.Msg
	buf := getBuffer()
	xbuf := buf.tmp[:0]

	if f.flag == LminFlags { // Minimal mode
		xbuf = append(xbuf, '<')
		itoa(&xbuf, int(e.Lvl), 1)
		xbuf = append(xbuf, '>')
		xbuf = append(xbuf, f.prefix...) // add any custom prefix

	} else {
		if f.flag&(Lshortfile|Llongfile) != 0 {
			if e.fok {
				file, line = e.FileInfo()
			} else {
				file = "???"
				line = 0
			}
		}
		if f.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
			now = e.Time()
		}
		f.formatHeader(&xbuf, e.Lvl, now, e.Name, file, line)
	}

	xbuf = append(xbuf, msg...)

	if len(e.Data) > 0 {
		xbuf = append(xbuf, ' ')
		marshalKeyvals(&buf.Buffer, e.Data...)
		xbuf = append(xbuf, buf.Buffer.Bytes()...)
	}

	// Finish up
	if len(msg) == 0 || msg[len(msg)-1] != '\n' {
		xbuf = append(xbuf, '\n')
	}

	// Now write the message to the tree of chained writers.
	// If the tree root is a EventWriter, provide the orignal event too.
	var err error
	if l, ok := f.out.(EvWriter); ok {
		_, err = l.EvWrite(e, xbuf)
	} else {
		_, err = f.out.Write(xbuf)
	}

	// release the buffer
	putBuffer(buf)

	return err
}

// strings.Map func for removing invalid logfmt key chars from a key
var prunef = func(r rune) rune {
	if r <= ' ' || r == '=' || r == '"' || r == utf8.RuneError {
		return -1
	}
	return r
}

// No reason to create another byte.Buffer when we already have one.
// So let's pass it to a custom version of MarshalKeyvals()
func marshalKeyvals(w io.Writer, keyvals ...interface{}) {
	if len(keyvals) == 0 {
		return
	}
	enc := logfmt.NewEncoder(w)
	for i := 0; i < len(keyvals); i += 2 {
		k, v := keyvals[i], keyvals[i+1]
		if l, ok := v.(Lazy); ok {
			v = l.evaluate()
		}
		err := enc.EncodeKeyval(k, v)
		if err != nil {
			// Try save the error
			switch err {
			case logfmt.ErrUnsupportedKeyType, logfmt.ErrNilKey:
				continue // Can't save - skip this pair
			case logfmt.ErrInvalidKey:
				if key, ok := k.(string); ok {
					// remove invalid chars and try again
					key = strings.Map(prunef, key)
					err = enc.EncodeKeyval(key, v)
				} else {
					continue // can't save this key
				}
			default:
				// value related error, replace with string form of error
				if _, ok := err.(*logfmt.MarshalerError); ok || err == logfmt.ErrUnsupportedValueType {
					v = err
					err = enc.EncodeKeyval(k, v)
				}
			}
		}
		if err != nil {
			enc.EncodeKeyval("ERROR", "ERROR")
		}
	}
	return
}

func (f *stdformatter) formatHeader(buf *[]byte, level syslog.Priority, t time.Time, name string, file string, line int) {

	if f.flag&(Llevel) != 0 {
		if f.flag&(Lcolor) != 0 {
			*buf = append(*buf,
				fmt.Sprintf("\x1b[%sm%s\x1b[0m",
					levelColors[level],
					(*f.pfxarr)[level])...)
		} else {
			*buf = append(*buf, (*f.pfxarr)[level]...) // level prefix
		}
	}

	*buf = append(*buf, f.prefix...) // add any custom prefix

	if name != "" && f.flag&(Lname) != 0 {
		*buf = append(*buf, '(')
		*buf = append(*buf, name...)
		*buf = append(*buf, ") "...)
	}

	if f.flag&(Ldate|Ltime|Lmicroseconds) != 0 {
		if f.flag&LUTC != 0 {
			t = t.UTC()
		}
		if f.flag&Ldate != 0 {
			year, month, day := t.Date()
			itoa(buf, year, 4)
			*buf = append(*buf, '/')
			itoa(buf, int(month), 2)
			*buf = append(*buf, '/')
			itoa(buf, day, 2)
			*buf = append(*buf, ' ')
		}
		if f.flag&(Ltime|Lmicroseconds) != 0 {
			hour, min, sec := t.Clock()
			itoa(buf, hour, 2)
			*buf = append(*buf, ':')
			itoa(buf, min, 2)
			*buf = append(*buf, ':')
			itoa(buf, sec, 2)
			if f.flag&Lmicroseconds != 0 {
				*buf = append(*buf, '.')
				itoa(buf, t.Nanosecond()/1e3, 6)
			}
			*buf = append(*buf, ' ')
		}
	}

	if f.flag&(Lpid) != 0 {
		*buf = append(*buf, '[')
		itoa(buf, pid, -1)
		*buf = append(*buf, "] "...)
	}

	if f.flag&(Lshortfile|Llongfile) != 0 {
		if f.flag&Lshortfile != 0 {
			short := file
			for i := len(file) - 1; i > 0; i-- {
				if file[i] == '/' {
					short = file[i+1:]
					break
				}
			}
			file = short
		}
		*buf = append(*buf, file...)
		*buf = append(*buf, ':')
		itoa(buf, line, -1)
		*buf = append(*buf, ": "...)
	}

}
