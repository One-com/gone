// Package reaper implements an IOActivityTimeout for a net.Conn by having
// a reaper go-routine monitoring connections for IO activity and close
// connections which fail to show activity.
//
// It supports net.Conn objects directly and if they are wrapped in *tls.Conn
//
package reaper

import (
	"crypto/tls"
	"errors"
	"net"
	"sync/atomic"
	"unsafe"
)

// IOActivityTimeout lets you toggle whether IOActivityTimeout is enabled
// for a net.Conn.
// It will work even if the net.Conn is wrapped as a *tls.Conn, but to do this
// requires an unsafe conversion relying on the underlying net.Conn being the first
// member of the tls.Conn struct.
// This is sadly necessary since crypto/tls.Conn doesn't expose the underlying
// net.Conn.
// Go 1.8 supports getting to the underlying net.Conn through ClientHelloInfo.
// That is however too late for this purpose.
func IOActivityTimeout(c net.Conn, enable bool) (success bool, err error) {

	var ioc *conn

	switch tc := c.(type) {
	case *tls.Conn:
		ptr := unsafe.Pointer(tc)
		cptr := (*net.Conn)(ptr)
		switch mc := (*cptr).(type) {
		case *conn:
			ioc = mc
		default:
			return
		}
	case *conn:
		ioc = tc
	default:
		return
	}

	err = ioActivityTimeout(ioc, enable)
	if err == nil {
		success = true
	}
	return
}

func ioActivityTimeout(c *conn, enable bool) (err error) {
	active := atomic.LoadUint64(&c.activeCount)
	// don't enable if conn closed
	if active&1 == 0 {
		if enable {
			c.ioActivityTimeoutEnabled.setTrue()
			//fmt.Println("TOGGLE true", c.ioActivityTimeoutEnabled.isSet())
		} else {
			c.ioActivityTimeoutEnabled.setFalse()
			//fmt.Println("TOGGLE false", c.ioActivityTimeoutEnabled.isSet())
		}
		return
	}
	return ErrClosed
}

// atomicBool is used for marking the IOActicityTimeout feature enabled.
type atomicBool int32

func (b *atomicBool) isSet() bool { return atomic.LoadInt32((*int32)(b)) != 0 }
func (b *atomicBool) setTrue()    { atomic.StoreInt32((*int32)(b), 1) }
func (b *atomicBool) setFalse()   { atomic.StoreInt32((*int32)(b), 0) }

// ErrClosed is returned if IOActivityTimeout is called on a closed connection
var ErrClosed = errors.New("IOActivityTimeout: closed connection")

type conn struct {
	net.Conn

	// additional booking for inactive connections
	// active is an atomic uint64; the low bit is whether Close has
	// been called. the rest of the bits are the number time IO operations
	// have been performed on the underlying net.Conn reference
	activeCount     uint64 // bumped on Read/Write
	lastActiveCount uint64 // the value when last checked
	reaperMiss      int64  // number of reaper cycles activity has not changed

	ioActivityTimeoutEnabled atomicBool
	next                     *conn
}

func (c *conn) Read(b []byte) (rc int, err error) {
	rc, err = c.Conn.Read(b)
	// apply an aggressive strategy and update I/O call count only on successful read
	if err == nil {
		atomic.AddUint64(&c.activeCount, 2)
	}
	return
}

func (c *conn) Write(b []byte) (rc int, err error) {
	rc, err = c.Conn.Write(b)
	// apply an aggressive strategy and update I/O call count only on successful write
	if err == nil {
		atomic.AddUint64(&c.activeCount, 2)
	}
	return
}

// Close closes the connection.
func (c *conn) Close() (err error) {
	var active uint64
	for {
		// Interlock last bit flag with other ongoing `Close()` function invocation
		active = atomic.LoadUint64(&c.activeCount)
		if atomic.CompareAndSwapUint64(&c.activeCount, active, active|1) {
			err = c.Conn.Close()
			break
		}
	}

	return
}

func (c *conn) tryClose() (connClosed bool) {
	// Interlock last bit flag with other ongoing active invokation of Close function call
	active := atomic.LoadUint64(&c.activeCount)
	// Another go routine aleady closed the connection
	if active&1 != 0 {
		connClosed = true
	} else if atomic.CompareAndSwapUint64(&c.activeCount, active, active|1) {
		// Close the connection as originally created.
		c.Conn.Close()
		connClosed = true
	}
	return
}
