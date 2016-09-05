package log

import (
	"io/ioutil"
	stdlog "log"
	"testing"
)

// Performance of the standard library to compare
func BenchmarkGoStdPrintln(b *testing.B) {
	const testString = "test"
	l := stdlog.New(ioutil.Discard, "", LstdFlags)
	for i := 0; i < b.N; i++ {
		l.Println(testString)
	}
}

// Similar with gonelog using stdformatter
func BenchmarkStdPrintln(b *testing.B) {
	const testString = "test"
	l := New(ioutil.Discard, "", LstdFlags)
	for i := 0; i < b.N; i++ {
		l.Println(testString)
	}
}

// Similar using flxformatter
func BenchmarkFlxPrintln(b *testing.B) {
	const testString = "test"
	h := NewStdFormatter(ioutil.Discard, "", LstdFlags)
	l := NewLogger(LvlDEFAULT, h)
	l.DoTime(true)
	for i := 0; i < b.N; i++ {
		l.Println(testString)
	}
}

// Try standard lib in parallel
func BenchmarkParallelGoStdPrintln(b *testing.B) {
	const testString = "test"
	l := stdlog.New(ioutil.Discard, "", LstdFlags)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Println(testString)
		}
	})
}

// similar but with gonelog
func BenchmarkParallelStdPrintln(b *testing.B) {
	const testString = "test"
	l := New(ioutil.Discard, "", LstdFlags)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Println(testString)
		}
	})
}

// similar but with flxformatter
func BenchmarkParallelFlxPrintln(b *testing.B) {
	const testString = "test"
	h := NewStdFormatter(ioutil.Discard, "", LstdFlags)
	l := NewLogger(LvlDEFAULT, h)
	l.DoTime(true)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Println(testString)
		}
	})
}

// standard API with minimal mode
func BenchmarkParallelMinPrintln(b *testing.B) {
	const testString = "test"
	h := NewMinFormatter(ioutil.Discard)
	l := NewLogger(LvlDEFAULT, h)
	l.DoTime(false)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			l.Println(testString)
		}
	})
}

// Using ERRORok() to log
func BenchmarkParallelMinERRORok(b *testing.B) {
	const testString = "test"
	h := NewMinFormatter(ioutil.Discard)
	l := NewLogger(LvlDEFAULT, h)
	l.DoTime(false)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if f, ok := l.ERRORok(); ok {
				f(testString)
			}
		}
	})
}

// Using DEBUGok() which will not be logged. - showing how cheap debug log statements can be.
func BenchmarkParallelMinDEBUGok(b *testing.B) {
	const testString = "test"
	h := NewMinFormatter(ioutil.Discard)
	l := NewLogger(LvlDEFAULT, h)
	l.DoTime(false)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			if f, ok := l.DEBUGok(); ok {
				f(testString)
			}
		}
	})
}
