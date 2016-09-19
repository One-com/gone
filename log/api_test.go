package log_test

import (
	"bytes"
	"github.com/One-com/gone/log"
	"github.com/One-com/gone/log/syslog"
	"os"
	"testing"
)

func output(l *log.Logger, msg string) {
	l.Output(2, msg)
}

func ExampleOutput() {
	l := log.New(os.Stdout, "", log.Lshortfile)
	l.DoCodeInfo(true)
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Lshortfile)
	log.SetPrefix("")

	output(l, "output1")
	log.Output(1, "output2")
	// Output:
	// api_test.go:22: output1
	// api_test.go:23: output2
}

//----------------------------------------------------------------

func ExampleMinimal() {
	log.Minimal() // change default Logger to not be stdlib compatible but minimal
	log.ERROR("fejl")
	// Output:
	// <3>fejl
}

func ExampleNewMinFormatter() {
	h := log.NewMinFormatter(log.SyncWriter(os.Stdout), log.PrefixOpt("PFX:"))
	l := log.NewLogger(syslog.LOG_WARNING, h)
	l.ERROR("fejl")
	// Output:
	// <3>PFX:fejl
}

// Compatible with std lib
func ExampleNew() {
	l := log.New(os.Stdout, "", log.Llevel)
	l.Println("notice")
	// Output:
	// <6>notice

}

func ExampleNewLogger() {
	h := log.NewStdFormatter(log.SyncWriter(os.Stdout), "", log.Llevel)
	l := log.NewLogger(syslog.LOG_WARNING, h)
	l.SetPrintLevel(syslog.LOG_NOTICE, false)

	// Traditional.
	// Evaluates arguments unless Lazy is used, but doesn't generate
	// Events above log level
	l.DEBUG("hej")
	l.INFO("hej")
	l.NOTICE("hej")
	l.WARN("hej")
	l.ERROR("hej")
	l.CRIT("hej")
	l.ALERT("hej")

	// Optimal
	// Doesn't do anything but checking the log level unless
	// something should be logged
	// A filtering handler would still be able to discard log events
	// based on level. Use Lazy to only evaluate just before formatting
	// Even by doing so a filtering writer might still discard the log line
	if f, ok := l.DEBUGok(); ok {
		f("dav")
	}
	if f, ok := l.INFOok(); ok {
		f("dav")
	}
	if f, ok := l.NOTICEok(); ok {
		f("dav")
	}
	if f, ok := l.WARNok(); ok {
		f("dav")
	}
	if f, ok := l.ERRORok(); ok {
		f("dav")
	}
	if f, ok := l.CRITok(); ok {
		f("dav")
	}
	if f, ok := l.ALERTok(); ok {
		f("dav")
	}

	// Primitive ... Allows for dynamically choosing log level.
	// Otherwise behaves like Traditional
	l.Log(syslog.LOG_DEBUG, "hop")
	l.Log(syslog.LOG_INFO, "hop")
	l.Log(syslog.LOG_NOTICE, "hop")
	l.Log(syslog.LOG_WARN, "hop")
	l.Log(syslog.LOG_ERROR, "hop")
	l.Log(syslog.LOG_CRIT, "hop")
	l.Log(syslog.LOG_ALERT, "hop")

	// Std logger compatible.
	// Will log with the default-level (default "INFO") - if that log-level is enabled.
	l.Print("default")
	// Fatal and Panic logs with level "ALERT"
	l.Fatal("fatal")
}

func ExampleSetPrintLevel() {
	l := log.GetLogger("my/lib")
	h := log.NewStdFormatter(log.SyncWriter(os.Stdout), "", log.Llevel|log.Lname)
	l.SetHandler(h)
	l.AutoColoring()
	l.SetLevel(syslog.LOG_ERROR)
	l.SetPrintLevel(syslog.LOG_NOTICE, false)

	l.Print("ignoring level")
	// Output:
	// <5> (my/lib) ignoring level

}

func ExampleWith() {
	l := log.GetLogger("my/lib")
	h := log.NewStdFormatter(log.SyncWriter(os.Stdout), "", log.Llevel|log.Lname)
	l.SetHandler(h)
	l.SetLevel(syslog.LOG_ERROR)

	l2 := l.With("key", "value")

	l3 := l2.With("more", "data")

	l3.ERROR("message")
	// Output:
	// <3> (my/lib) message more=data key=value

}

func ExampleGetLogger() {
	l := log.GetLogger("my/lib")
	h := log.NewStdFormatter(log.SyncWriter(os.Stdout), "", log.Llevel|log.Lname)
	l.SetHandler(h)
	l2 := log.GetLogger("my/lib/module")

	l3 := l2.With("k", "v")

	l3.NOTICE("notice")
	// Output:
	// <5> (my/lib/module) notice k=v
}

func ExampleHandler() {
	h := log.NewMinFormatter(log.SyncWriter(os.Stdout), log.PrefixOpt("PFX:"))
	l := log.GetLogger("mylog")
	l.SetHandler(h)
	l.ERROR("fejl")
	l.ApplyHandlerOptions(log.FlagsOpt(log.Llevel | log.Lname))
	l.WARN("advarsel")
	// Output:
	// <3>PFX:fejl
	// <4>PFX: (mylog) advarsel
}

// Test that Println() doesn't get k/v logging wrong
func TestEmptyPrintCreatesLineKV(t *testing.T) {
	var b bytes.Buffer
	ll := log.New(&b, "Header:", 0)
	l := ll.With("k", "v")
	l.Print()
	l.Println("non-empty")
	output := b.String()
	if output != "Header: k=v\nHeader:non-empty k=v\n" {
		t.Errorf("Println interferes with formatters newline conventions")
	}
}
