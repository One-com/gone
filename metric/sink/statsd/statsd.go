package statsd

import (
	"fmt"
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/num64"
	"io"
	"net"
	"os"
	"strconv"
	"sync"
	"time"
)

// Option is the type of configuration options for the statsd sink factory.
type Option func(*Sink) error

type unlockedSink struct {
	out    io.Writer
	max    int
	prefix string
	buf    []byte
	strip  bool
}

// Sink is a go-routine safe version of a Statsd sink.
// Call UnlockedSink() to get a faster, but not go-routine safe Sink.
type Sink struct {
	unlockedSink
	mu sync.Mutex
}

// Buffer sets the package size with which writes to the underlying io.Writer (often an UDPConn)
// is done.
func Buffer(size int) Option {
	return Option(func(s *Sink) error {
		s.max = size
		return nil
	})
}

// Prefix is prepended with "prefix." to all metric names
func Prefix(pfx string) Option {
	return Option(func(s *Sink) error {
		s.prefix = pfx + "."
		return nil
	})
}

// Peer is the address of the statsd UDP server
func Peer(addr string) Option {
	return Option(func(s *Sink) error {
		conn, err := net.DialTimeout("udp", addr, time.Second)
		if err != nil {
			return err
		}
		s.out = conn
		s.strip = true
		return nil
	})
}

// Output sets an general io.Writer as output instead of a UDPConn.
func Output(w io.Writer) Option {
	return Option(func(s *Sink) error {
		s.out = w
		return nil
	})
}

// New creasted a SinkFactory which is used to create Sinks sending data to a UDP statsd server
// Provide a UDP address, a prefix and a maximum UDP datagram size.
// 1432 should be a safe size for most nets.
func New(opts ...Option) (sink metric.Sink, err error) {

	s := &Sink{unlockedSink: unlockedSink{out: os.Stdout}}

	for _, o := range opts {
		err = o(s)
		if err != nil {
			return nil, err
		}
	}

	sink = s

	return
}

func (s *Sink) UnlockedSink() metric.Sink {
	newsink := &unlockedSink{}
	*newsink = s.unlockedSink // the buffer might race here, but we discard it.
	// give this sink its own buffer
	newsink.buf = make([]byte, 0, 512)
	return newsink
}

// Record a value with the sink
func (s *Sink) Record(mtype int, name string, value interface{}) {
	s.mu.Lock()
	s.unlockedSink.Record(mtype, name, value)
	s.mu.Unlock()
}

func (s *unlockedSink) Record(mtype int, name string, value interface{}) {
	curbuflen := len(s.buf)
	s.buf = append(s.buf, s.prefix...)
	s.buf = append(s.buf, name...)
	s.buf = append(s.buf, ':')
	switch v := value.(type) {
	case string:
		s.buf = append(s.buf, v...)
	case fmt.Stringer:
		s.buf = append(s.buf, v.String()...)
	default:
		panic("Not stringable")
	}
	s.buf = append(s.buf, '|')
	s.appendType(mtype)
	// sampe rate not supported
	s.buf = append(s.buf, '\n')
	s.flushIfBufferFull(curbuflen)
}

// Record a Numeric64 value with the sink
func (s *Sink) RecordNumeric64(mtype int, name string, value num64.Numeric64) {
	s.mu.Lock()
	s.unlockedSink.RecordNumeric64(mtype, name, value)
	s.mu.Unlock()
}

func (s *unlockedSink) RecordNumeric64(mtype int, name string, value num64.Numeric64) {
	curbuflen := len(s.buf)
	s.buf = append(s.buf, s.prefix...)
	s.buf = append(s.buf, name...)
	s.buf = append(s.buf, ':')
	s.appendNumeric64(value)
	s.buf = append(s.buf, '|')
	s.appendType(mtype)
	// sample rate not supported
	s.buf = append(s.buf, '\n')
	s.flushIfBufferFull(curbuflen)
}

func (s *Sink) Flush() {
	s.mu.Lock()
	s.unlockedSink.Flush()
	s.mu.Unlock()
}

func (s *unlockedSink) Flush() {
	s.flush(0)
}

func (s *unlockedSink) flushIfBufferFull(lastSafeLen int) {
	if len(s.buf) > s.max {
		s.flush(lastSafeLen)
	}
}

func (s *unlockedSink) flush(n int) {
	if len(s.buf) == 0 {
		return
	}
	if n == 0 {
		n = len(s.buf)
	}

	// Trim the last \n, StatsD does not like it.
	if s.strip {
		s.out.Write(s.buf[:n-1])
	} else {
		s.out.Write(s.buf[:n])
	}

	if n < len(s.buf) {
		copy(s.buf, s.buf[n:])
	}
	s.buf = s.buf[:len(s.buf)-n]
}

func (s *unlockedSink) appendType(t int) {
	switch t {
	case metric.MeterGauge:
		s.buf = append(s.buf, 'g')
	case metric.MeterCounter:
		s.buf = append(s.buf, 'c')
	case metric.MeterTimer, metric.MeterHistogram: // until we are sure the statsd server supports otherwise
		s.buf = append(s.buf, "ms"...)
	case metric.MeterSet:
		s.buf = append(s.buf, 's')

	}
}

func (s *unlockedSink) appendNumeric64(v num64.Numeric64) {
	switch v.Type {
	case num64.Uint64:
		s.buf = strconv.AppendUint(s.buf, v.Uint64(), 10)
	case num64.Int64:
		s.buf = strconv.AppendInt(s.buf, v.Int64(), 10)
	case num64.Float64:
		s.buf = strconv.AppendFloat(s.buf, v.Float64(), 'f', -1, 64)
	}
}

func (s *unlockedSink) appendNumber(v interface{}) {
	switch n := v.(type) {
	case int:
		s.buf = strconv.AppendInt(s.buf, int64(n), 10)
	case uint:
		s.buf = strconv.AppendUint(s.buf, uint64(n), 10)
	case int64:
		s.buf = strconv.AppendInt(s.buf, n, 10)
	case uint64:
		s.buf = strconv.AppendUint(s.buf, n, 10)
	case int32:
		s.buf = strconv.AppendInt(s.buf, int64(n), 10)
	case uint32:
		s.buf = strconv.AppendUint(s.buf, uint64(n), 10)
	case int16:
		s.buf = strconv.AppendInt(s.buf, int64(n), 10)
	case uint16:
		s.buf = strconv.AppendUint(s.buf, uint64(n), 10)
	case int8:
		s.buf = strconv.AppendInt(s.buf, int64(n), 10)
	case uint8:
		s.buf = strconv.AppendUint(s.buf, uint64(n), 10)
	case float64:
		s.buf = strconv.AppendFloat(s.buf, n, 'f', -1, 64)
	case float32:
		s.buf = strconv.AppendFloat(s.buf, float64(n), 'f', -1, 32)
	}
}

/* Some of the above code has been borrowed from github.com/alexcesaro/statsd

... which carries the license:

The MIT License (MIT)

Copyright (c) 2015 Alexandre Cesaro

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of
the Software, and to permit persons to whom the Software is furnished to do so,
subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS
FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR
COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER
IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN
CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
*/
