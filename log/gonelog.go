package log

import (
	"github.com/One-com/gone/log/syslog"
	"sync/atomic"
)

// LvlDEFAULT is the default log level at which Print*() functions will log.
// Leveled logging is provided with the 8 syslog levels
// Additional 2 pseudo-levels ("Fatal"/ "Panic") which log at Alert-level, but have
// side-effects like the stdlogger. (os.Exit(1)/panic())
// Print*() functions will produce log-events with a "default" level.
const LvlDEFAULT syslog.Priority = syslog.LOG_INFO

// lconfig  holds variables which could potentially change during the life of
// a Logger and which needs sync/atomic ops for access, so we can't allow the
// user to copy them, when a logger is copied.
type lconfig struct {
	config uint32 // atomic data (should be cheap since it's an 32-bit well aligned)
}

// lconfig uint32 mask
const (
	levelshift = 3

	// Use the low 8 bit for the basic stuff.
	// 3 bit loglevel, 3 bit defaultlevel, 1 bit dotime, 1 bit docode
	// This leaves potentially 24 bit for something else - like log-categories
	maskLogLvl uint32 = 0x00000007 // The log level determining which events are generated
	maskDefLvl uint32 = 0x00000038 // The log level for Print*() statements
	maskDoCode uint32 = 0x00000040 // attach file/line info to the events.
	maskDoTime uint32 = 0x00000080 // pre-timestamp events.

	maskDefObl uint32 = 0x00000100 // Generate Print*() events despite log level.

	// The default logger has default level and Print*() logging will *not* obey levels.
	defConfig uint32 = (uint32(LvlDEFAULT) << levelshift) | uint32(LvlDEFAULT) | maskDefObl
)

// Logger implements a stdlib *log.Logger source code compatible interface and extra
// methods for supporting leveled, structured and context logging.
//
// Don't create these your self. Use a constructor function.
// Once created, its member attributes cannot be changed (to avoid races),
// Exceptions are:
// * Changing the config, which goes through atomic operations
// * Changing the handler, which can be atomically replaced.
//
// There's a race between changing both, so beware! If you swap in a handler which
// does file/line-recording without changing config to DoCodeInfo() first,
// there will no code-info in the log-events during the race.
// Repeat: There's no way to change handler and config atomically together.
// So don't enable a handler using something the config doesn't support (timestamps/codeinfo)
// unless you can live with potentially a few log lines with wrong content.
// All attributes are reference-like attributes. Copying them will not gain you anything, but effectively just having two "pointers" to the same logger.
// Copying values of this type is discouraged, but in principle legal, since the direct members never change.
// The new copy will behave like the old, and modifications to one of them will affect the other.
//
// Logger allows for contextual logging, by keeping K/V data to be logged with all
// events and to create sub child loggers with additional K/V data.
type Logger struct {
	// This logger is optionally part of a tree based on it's name.
	//  ("a/b/c") ...placing it in a global hierarchy.
	// The root Logger of the hierarchy is not named.
	name string

	// pointer to the Logger config.
	cfg *lconfig

	// An atomic swappable handle to the loghandler and any name-based parent
	h *swapper

	// The Logger is a context-child of another wrt. K/V data created by With().  This is *NOT* the name based parent.
	cparent *Logger

	// K/V Attributes common to all events logged ... Using a slice instead of map for speed
	data []interface{}
}

// NewLogger creates a new unamed Logger out side of the named Logger hierarchy.
func NewLogger(level syslog.Priority, handler Handler) (l *Logger) {

	i := defConfig & ^maskLogLvl | (uint32(level) & maskLogLvl)
	c := &lconfig{config: i}
	l = &Logger{
		name: "", // not a part of hierarchy
		h:    newSwapper(),
		cfg:  c,
	} // nil cparent
	l.h.SwapHandler(handler)
	return l
}

// newLogger Creates a new Logger.
// Not exported, since applications should use GetLogger() to get Loggers with a name.
// Once created and the pointer is returned, the only thing which can be changed in this object is
// in the config/swapper - via accessor methods. This ensures it's go-routine safe
func newLogger(name string) (l *Logger) {

	c := &lconfig{config: defConfig}
	l = &Logger{
		name: name,
		h:    newSwapper(),
		cfg:  c,
	}
	return
}

// Unconditionaly logs an event
// Some logging goes through here to keep the same number of stackframes
// for all calls to let newEvent() know how to do code info
func (l *Logger) log(level syslog.Priority, msg string, kv ...interface{}) error {
	var e *event
	if kv == nil {
		e = l.newEvent(level, msg, nil)
	} else {
		e = l.newEvent(level, msg, normalize(kv))
	}
	return l.h.Log(e)
}

// Unconditionaly logs an event
// Some logging goes through here to keep the same number of stackframes
// for all calls to let newEvent() know how to do code info.
// Provides support for stdlib Output() compatibility.
func (l *Logger) output(calldepth int, s string) error {
	return l.h.Log(l.calldepthEvent(l.DefaultLevel(), calldepth, s))
}

// SetHandler atomically swaps in a different root of the Handler tree
func (l *Logger) SetHandler(h Handler) {
	l.h.SwapHandler(h)
}

// ApplyHandlerOptions clones the current Handles and tries to apply the supplied
// HandlerOptions to the clone - then swaps in the clone atomically to not loose
// Log events.
// Supplied HandlerOptions must be compatible with the current Handler, or the Handler
// will/(should) reject it and treat it as a No-Op.
// If the attached Handler does not support Cloning or HandlerOptions and/or you are
// using a more complext Handler hierarchy, you should probably create the new Handler
// Manually and use SetHandler()
func (l *Logger) ApplyHandlerOptions(opt ...HandlerOption) {
	l.h.ApplyHandlerOptions(opt...)
}

// Autocoloring asks the current Handler to test if there's a TTY attached to an
// output and if so, apply coloring to the formatter.
func (l *Logger) AutoColoring() {
	l.h.AutoColoring()
}

// With ties a sub-Context to the Logger.
func (l *Logger) With(kv ...interface{}) *Logger {
	d := normalize(kv)
	// copy the pointers to handler and config to ease access later
	// For all purposes except data, this child will be the same as it's cparent.
	new := &Logger{
		name: l.name,
		cfg:  l.cfg,
		h:    l.h,
		// Limiting the capacity of the stored keyvals ensures that a new
		// backing array is created if the slice must grow
		// Using the extra capacity without copying risks a data race that
		// would violate the Logger interface contract.
		data:    d[:len(d):len(d)],
		cparent: l,
	}
	return new
}

// DoTime tries to turn on or off timestamping.
// It can fail if some other go-routine simultaneous is manipulating the config.
// If the generated log-events are not timestamped on creation some formatters
// will create their own timestamp anyway.
// Having this global option saves time.Now() calls if no one is using the time info
// (which is the case for minimal logging). It also enables using a single timestamp
// for all formatting of the log event.
// Returning whether the change was successful
func (l *Logger) DoTime(doTime bool) bool {
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	if doTime {
		n = c | maskDoTime
	} else {
		n = c & ^maskDoTime
	}
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// DoCodeInfo tries to turn on or off registering the file and line of the log call.
// Formatters which try to log this info will not give meaningful info if this is turned off.
// It can fail if some other go-routine simultaneous is manipulating the config.
// Returning whether the change was successful
func (l *Logger) DoCodeInfo(doCode bool) bool {
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	if doCode {
		n = c | maskDoCode
	} else {
		n = c & ^maskDoCode
	}
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// IncLevel tries to increase the log level
func (l *Logger) IncLevel() bool {
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	n = (c & maskLogLvl)
	if n >= uint32(syslog.LOG_DEBUG) {
		n = uint32(syslog.LOG_DEBUG)
	} else {
		n++
	}
	n = (c & ^maskLogLvl) | n
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// DecLevel tries to decrease the log level
func (l *Logger) DecLevel() bool {
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	n = (c & maskLogLvl)
	if n == uint32(syslog.LOG_EMERG) {
		n = uint32(syslog.LOG_EMERG)
	} else {
		n--
	}
	n = (c & ^maskLogLvl) | n
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// SetLevel set the Logger log level.
// returns success
func (l *Logger) SetLevel(level syslog.Priority) bool {
	if level > syslog.LOG_DEBUG {
		level = syslog.LOG_DEBUG
	}
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	n = (c & ^maskLogLvl) | uint32(level)
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// Deprecated: Use SetPrintLevel()
func (l *Logger) SetDefaultLevel(level syslog.Priority, respect bool) bool {
	return l.SetPrintLevel(level, respect)
}

// SetPrintLevel sets the level which Print*() methods are logging with.
// "respect" indicated whether Print*() statements will respect the Logger loglevel
// or generate events anyway. (with the default log level).
// Without "respect" the logger can generate events above its loglevel. Such events
// can however still be filtered out by filter-handler, or filter-writers, or by external
// systems like syslog.
// returns success
func (l *Logger) SetPrintLevel(level syslog.Priority, respect bool) bool {
	if level > syslog.LOG_DEBUG {
		level = syslog.LOG_DEBUG
	}
	c := atomic.LoadUint32(&l.cfg.config)
	var n uint32
	n = (c & ^maskDefLvl) | (uint32(level) << levelshift)
	if respect {
		n &^= maskDefObl
	} else {
		n |= maskDefObl
	}
	return atomic.CompareAndSwapUint32(&l.cfg.config, c, n)
}

// Does returns whether the Logger would generate an event at this level?
// This can be used for optimal performance logging
func (l *Logger) Does(level syslog.Priority) bool {
	return level <= l.cfg.level()
}

// Do is Setlevel() - For completeness
// returns success
func (l *Logger) Do(level syslog.Priority) bool {
	return l.SetLevel(level)
}

/**********  methods returning the current config ************/

// DoingPrintLevel returns whether a log.Println() would actually
// generate a log event with the current config.
// It's equivalent to l.Does(l.PrintLevel()) - but atomically
func (l *Logger) DoingPrintLevel() (syslog.Priority, bool) {
	return l.cfg.doingDefaultLevel()
}

// DoingDefaultLevel returns whether a log.Println() would actually
// generate a log event with the current config.
// It's equivalent to l.Does(l.DefaultLevel()) - but atomically
// Deprecated, use DoingPrintLevel()
func (l *Logger) DoingDefaultLevel() (syslog.Priority, bool) {
	return l.cfg.doingDefaultLevel()
}

// Level returns the current log level
func (l *Logger) Level() syslog.Priority {
	return l.cfg.level()
}

// PrintLevel returns the current log level of Print*() methods
func (l *Logger) PrintLevel() syslog.Priority {
	return l.cfg.defaultLevel()
}

// DefaultLevel returns the current log level of Print*() methods. Deprecated, use PrintLevel()
func (l *Logger) DefaultLevel() syslog.Priority {
	return l.cfg.defaultLevel()
}

// DoingTime returns whether the Logger is currently timestamping all events on
// creation
func (l *Logger) DoingTime() bool {
	return l.cfg.doingTime()
}

// DoingCodeInfo returns whether the Logger is currently recording file/line info
// for all log events
func (l *Logger) DoingCodeInfo() bool {
	return l.cfg.doingCode()
}

/********************** lconfig operations *************************/

func (lc *lconfig) clone() *lconfig {
	return &lconfig{
		config: atomic.LoadUint32(&lc.config),
	}
}

func (lc *lconfig) level() (l syslog.Priority) {
	c := atomic.LoadUint32(&lc.config)
	l = syslog.Priority(c & maskLogLvl)
	return
}

func (lc *lconfig) defaultLevel() (l syslog.Priority) {
	c := atomic.LoadUint32(&lc.config)
	l = syslog.Priority(c & maskDefLvl >> levelshift)
	return
}

func (lc *lconfig) doingDefaultLevel() (syslog.Priority, bool) {
	c := atomic.LoadUint32(&lc.config)
	l := syslog.Priority(c & maskLogLvl)
	d := syslog.Priority((c & maskDefLvl) >> levelshift)
	respect := (c & maskDefObl) == 0
	return d, ((d <= l) || !respect)
}

func (lc *lconfig) doingTime() bool {
	c := atomic.LoadUint32(&lc.config)
	return c&maskDoTime != 0
}

func (lc *lconfig) doingCode() bool {
	c := atomic.LoadUint32(&lc.config)
	return c&maskDoCode != 0
}

func (lc *lconfig) doing() (time, code bool) {
	c := atomic.LoadUint32(&lc.config)
	return (c&maskDoTime != 0), (c&maskDoCode != 0)
}
