package log

import (
	"github.com/One-com/gone/log/syslog"
	"github.com/One-com/gone/log/term"
	"io"
	"sync"
)

// some implementors of io.Writer for use in Formatters

// MaybeTtyWriter is a writer which know whether the underlying writer is a TTY
type MaybeTtyWriter interface {
	IsTty() bool
	io.Writer
}

// EvWriter is a Writer which wants to know the original event of the []byte buffer (for filtering)
// Is has to also be a io.Writer, else it can't be used in a formatter.
type EvWriter interface {
	EvWrite(e Event, b []byte) (n int, err error)
	io.Writer
}

// Note - The std lib alread had io.MultiWriter

//---
type writerFunc func(b []byte) (n int, err error)

// WriterFunc makes an io.Writer out of a function by calling it on Write()
func WriterFunc(fn func(b []byte) (n int, err error)) io.Writer {
	return writerFunc(fn)
}

func (w writerFunc) Write(b []byte) (n int, err error) {
	return w(b)
}

//---
type evWriterFunc func(e Event, b []byte) (n int, err error)

// EventWriterFunc created a new EvWriter from a function taking the event and
// a formattet logline. Such functions need to be aware that e can be nil
func EventWriterFunc(fn func(e Event, b []byte) (n int, err error)) EvWriter {
	return evWriterFunc(fn)
}
func (w evWriterFunc) EvWrite(e Event, b []byte) (n int, err error) {
	return w(e, b)
}
func (w evWriterFunc) Write(b []byte) (n int, err error) {
	return w(Event{nil}, b) // pretend we don't know the loglevel and just set it to highest
}

/*======================================================*/

// syncWriter allows to synchronize writes to an io.Writer and implements MaybeTtyWriter
type syncWriter struct {
	mu    sync.Mutex
	out   io.Writer
	istty bool
}

// SyncWriter encapsulates an io.Writer in a Mutex, so only one Write operation is done
// at a time.
func SyncWriter(w io.Writer) io.Writer {
	s := &syncWriter{out: w}
	if term.IsTty(w) {
		s.istty = true
	}
	return s
}

func (s *syncWriter) IsTty() bool {
	return s.istty
}

func (s *syncWriter) Write(b []byte) (n int, err error) {
	s.mu.Lock()
	n, err = s.out.Write(b)
	s.mu.Unlock()
	return
}
func (s *syncWriter) SetOutput(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if term.IsTty(w) {
		s.istty = true
	}
	s.out = w
}

//func SyncWriter(w io.Writer) io.Writer {
//      var mu sync.Mutex
//	return WriterFunc(func (b []byte) (n int, err error) {
//		mu.Lock()
//		defer mu.Unlock()
//		return w.Write(b)
//	})
//}

type multiEvWriter struct {
	writers []EvWriter
}

// Dummy method to allow a multiEvWriter to behave like an io.MultiWriter
func (t *multiEvWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}
func (t *multiEvWriter) EvWrite(e Event, p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.EvWrite(e, p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

// MultiEventWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command, providing all
// with the original event to do filtering.
func MultiEventWriter(writers ...EvWriter) EvWriter {
	w := make([]EvWriter, len(writers))
	copy(w, writers)
	return &multiEvWriter{w}
}

// EventWriter creates a dummy EvWriter from an io.Writer
func EventWriter(w io.Writer) EvWriter {
	return EventWriterFunc(func(e Event, b []byte) (n int, err error) {
		return w.Write(b)
	})
}

/// and finally:

// LevelFilterWriter creates a filtering EventWriter which writes whats below (or equal) max level to
// The underlying io.Writer
func LevelFilterWriter(max syslog.Priority, w io.Writer) EvWriter {
	return EventWriterFunc(func(e Event, b []byte) (n int, err error) {
		if e != (Event{}) && e.Lvl > max {
			return 0, nil
		}
		return w.Write(b)
	})
}
