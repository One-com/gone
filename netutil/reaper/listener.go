package reaper

import (
	"net"
	"sync/atomic"
	"time"
)

// A net.Listener wrapper which returns connetions keeping track of successful read/write calls
// A monitor go-routine closes connection if no I/O activity happens for a designated duration
type listener struct {
	net.Listener
	newChan  chan *conn
	interval time.Duration
	maxMiss  int64
	reapers  uint32
}

// NewIOActivityTimeoutListener wraps net.Listener with IOActivityTimeout functionality, so the accepted
// connections can use IOActivityTimeout() to enable it.
// The returned listener will not enable IOActivityTimeout per default.
func NewIOActivityTimeoutListener(orig net.Listener, timeout, reaperInterval time.Duration) (l net.Listener) {

	if timeout < reaperInterval {
		timeout = reaperInterval
	}

	// The number of reaper runs without an activity update a connections is allowed to have.
	maxReaperMiss := timeout.Nanoseconds() / reaperInterval.Nanoseconds()

	l = &listener{
		Listener: orig,
		newChan:  make(chan *conn),
		interval: reaperInterval,
		maxMiss:  maxReaperMiss,
	}

	return
}

func (l *listener) Accept() (rc net.Conn, err error) {
	var c net.Conn
	c, err = l.Listener.Accept()
	if err == nil {
		ic := &conn{Conn: c}
		
	HANDOFF:
		for {
			select {
			case l.newChan <- ic:
				break HANDOFF
			default:
				r := atomic.LoadUint32(&l.reapers)
				if r < 2 {
					atomic.AddUint32(&l.reapers, 1)
					go reaper(ic, l.newChan, l.interval, l.maxMiss, &l.reapers)
					break HANDOFF
				}
			}
		}
		rc = ic
	}
	return
}
