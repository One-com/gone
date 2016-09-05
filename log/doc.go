/*
Package gone/log is a drop-in replacement for the standard Go logging library "log" which is fully source code compatible support all the standard library API while at the same time offering advanced logging features through an extended API.

The design goals of gonelog was:

    * Standard library source level compatibility with mostly preserved behaviour.
    * Leveled logging with syslog levels.
    * Structured key/value logging
    * Hierarchical contextable logging to have k/v data in context logged automatically.
    * Low resource usage to allow more (debug) log-statements even if they don't result in output.
    * Light syntax to encourage logging on INFO/DEBUG level. (and low cost of doing so)
    * Flexibility in how log events are output.
    * A fast simple lightweight default in systemd newdaemon style only outputting <level>message
      to standard output.
    * Facilitating configuring logging for libraries by the application.

Synopsis

Out of the box the default logger with package level methods works like the standard library *log.Logger with all the standard flags and methods:

    import "github.com/One-com/gonelog/log"

    log.Println("Hello log")

    mylog := log.New(os.Stdout,"PFX:",LstdFlags)
    mylog.Fatal("Arggh")

Under the hood the default *log.Logger is however a log context object which can have key/value data and which generate log events with a syslog level and hands them of to a log Handler for formatting and output.

The default Logger supports all this as well, using log level constants source code compatible with the "log/syslog" package through the github.com/One-com/gone/log/syslog package:

	package syslog
	const (
		// Severity.

		// From /usr/include/sys/syslog.h.
		// These are the same on Linux, BSD, and OS X.
		LOG_EMERG Priority = iota
		LOG_ALERT
		LOG_CRIT
		LOG_ERR
		LOG_WARNING
		LOG_NOTICE
		LOG_INFO
		LOG_DEBUG
	)

Logging with key/value data is (in its most simple form) done by calling level specific functions. First argument is the message, subsequence arguments key/value data:

    	log.DEBUG("hi", "key", "value")
	log.INFO("hi again")
	log.NOTICE("more insisting hi")
	log.WARN("Hi?")
	log.ERROR("Hi!", "key1","value1", "key2", "value2")
	log.CRIT("HI! ")
	log.ALERT("HEY THERE, WAKE UP!")

Earch *log.Logger object has a current "log level" which determines the maximum log level for which events are actually generated. Logging above that level will be ignored. This log level can be controlled:

      log.SetLevel(syslog.LOG_WARN)
      log.DecLevel()
      log.IncLevel()


Calling Fatal*() and Panic*() will in addition to Fataling/panicing log at level ALERT.

The Print*() methods will log events with a configurable "default" log level - which default to INFO.

Per default the Logger *will* generate log event for Print*() calls even though the log level is lower. The Logger can be set to respect the actual log level also for Print*() statements by the second argument to SetPrintLevel()

	log.SetPrintLevel(syslog.LOG_NOTICE,true)

A new custom Logger with its own behavior and formatting handler can be created:

  	h := log.NewStdFormatter(os.Stdout,"",log.LstdFlags|log.Llevel|log.Lpid|log.Lshortfile)
	l := log.NewLogger(syslog.LOG_ERROR, h)
	l.DoTime(true)
	l.DoCodeInfo(true)

A customer Logger will not per default spend time timestamping events or registring file/line information. You have to enable that explicitly (it's not enabled by setting the flags on a formatting handler).

When having key/value data which you need to have logged in all log events, but don't want to remember put into every log statement, you can create a "child" Logger:

     reqlog := l.With( "session", uuid.NewUUID() )
     reqlog.ERROR("Invalid session")

To simply set the standard logger in a minimal mode where it only outputs <level>message to STDOUT and let an external daemon supervisor/log system do the rest (including timestamping) just do:

   log.Minimal()

Having many log statements can be expensive. Especially if the arguments to be logged are resource intensive to compute and there's no log events generated anyway.

There are 2 ways to get around that. The first is do do Lazy evaluation of arguments:

      	log.WARN("heavy", "fib123", log.Lazy(func() interface{} {return fib(123)} ))

The other is to pick an comma-ok style log function:

    	if f,ok := l.DEBUGok(); ok  { f("heavy", "fib123", fib(123)) }

Loggers can have names, placing them in a global "/" separated hierarchy.

It's recommended to create a Logger by mentioning it by name using GetLogger("logger/name") - instead of creating unnamed Loggers with NewLogger().
If such a logger exists you will get it returned, so you can configure it and set the formatter/output. Otherwise a new logger by that name is created. Libraries are encouraged to published the names of their Loggers and to name Loggers after their Go package. This works exactly like the Python "logging" library - with one exception:
When Logging an event at a Logger the tree of Loggers by name are only traversed towards to root to find the first Logger having a Handler attached, not returning an error. The log-event is then sent to that handler. If that handler returns an error, the parent Logger and its Handler is tried. This allows to contruct a "Last Resort" parent for errors in the default log Handler.
The Python behaviour is to send the event to all Handlers found in the Logger tree. This is not the way it's done here. Only one Handler will be given the event to log. If you wan't more Handlers getting the event, use a MultiHandler.

	package main

	import (
	       "mylib" // which logs to a gonelog *log.Logger
	       "github.com/One-com/gone/log"
       	       "github.com/One-com/gone/log/syslog"
	       "os"
	)

	func main() {
	     log.SetLevel(syslog.WARN) // application will log at warn level
	     log.GetLogger("mylib").SetLevel(syslog.LOG_ERROR) // mylib will log at error level
	     log.SetOutput(os.Stderr) // Log events from mylib will be propagated up
	     mylib.FuncWhichLogsOnError()
	}


Happy logging.


*/
package log
