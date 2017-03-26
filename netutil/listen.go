// Package listen only defines interfaces for objects which can return listeners.
package netutil

import (
	"net"
)

// StreamListener - an object which can create a slice of listeners when invoked.
type StreamListener interface {
	Listen() (listeners []net.Listener, err error)
}

// PacketListener - an object which can create a slice of PacketConn listeners when invoked
type PacketListener interface {
	Listen() (listeners []net.PacketConn, err error)
}
