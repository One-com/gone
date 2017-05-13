package reaper

import (
	"context"
	"net"
	"sync/atomic"
	"time"
	//"fmt"
)

// Dialer implements Dial* functions for connections
// which can be monitored by a IOActivityTimeout reaper
type Dialer struct {
	dialer          *net.Dialer
	newChan         chan *conn
	interval        time.Duration
	maxMiss         int64
	reapers         uint32
	enableByDefault bool
}

// NewIOActivityTimeoutDialer wraps a *net.Dialer with potential IOActivityTimeout monitoring.
// - only enabled by default if requested.
func NewIOActivityTimeoutDialer(orig *net.Dialer, timeout, reaperInterval time.Duration, enableByDefault bool) (d *Dialer) {

	if timeout < reaperInterval {
		timeout = reaperInterval
	}

	// The number of reaper runs without an activity update a connections is allowed to have.
	maxReaperMiss := timeout.Nanoseconds() / reaperInterval.Nanoseconds()

	d = &Dialer{
		dialer:          orig,
		newChan:         make(chan *conn),
		interval:        reaperInterval,
		maxMiss:         maxReaperMiss,
		enableByDefault: enableByDefault,
	}
	return
}

// Dial implements a Dial function like *net.Dialer
func (d *Dialer) Dial(network, address string) (rc net.Conn, err error) {
	var c net.Conn
	c, err = d.dialer.Dial(network, address)
	if err == nil {
		rc = d.wrap_n_handoff(c)
	}
	return
}

// DialContext implements a DialContext function like *net.Dialer
func (d *Dialer) DialContext(ctx context.Context, network, address string) (rc net.Conn, err error) {
	var c net.Conn
	c, err = d.dialer.DialContext(ctx, network, address)
	if err == nil {
		rc = d.wrap_n_handoff(c)
	}
	return
}

func (d *Dialer) wrap_n_handoff(c net.Conn) (rc net.Conn) {

	ic := &conn{Conn: c}
	if d.enableByDefault {
		// Enable the IOActivityTimeout for each backend connection as Transport.Dailer
		IOActivityTimeout(ic, true)
	}
HANDOFF:
	for {
		select {
		case d.newChan <- ic:
			break HANDOFF
		default:
			r := atomic.LoadUint32(&d.reapers)
			if r < 2 {
				atomic.AddUint32(&d.reapers, 1)
				go reaper(ic, d.newChan, d.interval, d.maxMiss, &d.reapers)
				//fmt.Println("DID HANDOFF", d.maxMiss)
				break HANDOFF
			}
		}
	}
	rc = ic
	return
}
