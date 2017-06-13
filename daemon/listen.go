package daemon

import (
	"crypto/tls"
	"errors"
	"net"

	"github.com/One-com/gone/sd"
)

// ErrNoListener is returned from Listen() when a specified
// required inherited socket to listen on is not found.
var ErrNoListener = errors.New("No Matching Listener")

// ListenerSpec describes the properties of a listener so it can be instantiated
// either via the "sd" library or directly from stdlib package "net"
type ListenerSpec struct {
	Net string

	Addr string
	// ListenerFdName can be set to pick a named file descriptor as
	// Listener via LISTEN_FDNAMES
	// It is updated to contain the name of the chosen file descriptor
	// - if any
	ListenerFdName string

	// Extra sd.FileTest to apply to the listener inherited.
	ExtraFileTests []sd.FileTest

	// InheritOnly set to true requires the Listener to be inherited via
	// the environment and there will not be created a fresh Listener.
	InheritOnly bool

	// PrepareListener provides a callback to do last minute modifications of
	// the chosen listener. (like wrapping it in something else)
	// It will be called as a callback with the listener chosen before it's set.
	// The returned listener is set instead - wrapped in any TLS if
	// there's a TLSConfig set.
	PrepareListener func(net.Listener) net.Listener

	TLSConfig *tls.Config
}

// ListenerGroup implement a gone/daemon/listen interface using the gone/sd library.
type ListenerGroup []ListenerSpec

// Listen will create new listeners based on ListenerSpec, first trying to inherit
// a listener socket from the gone/sd library, and possibly, - if that fails, create
// a new listener via the stdlib net package. All listerners are Exported by the sd lib.
func (lg ListenerGroup) Listen() (listeners []net.Listener, err error) {

	// Close any already listening listeners on error exit
	defer func() {
		if err != nil {
			for _, l := range listeners {
				l.Close()
			}
		}
	}()

	for _, ls := range lg {

		name := ls.ListenerFdName
		var ln net.Listener
		var basictest sd.FileTest

		var taddr *net.TCPAddr
		var uaddr *net.UnixAddr

		var nett string = ls.Net
		if nett == "" { // default to TCP
			nett = "tcp"
		}

		switch nett {
		case "tcp", "tcp4", "tcp6":
			if ls.Addr != "" {
				taddr, err = net.ResolveTCPAddr(nett, ls.Addr)
				if err != nil {
					return
				}
			}
			basictest = sd.IsTCPListener(taddr)
		case "unix", "unixpacket":

			if ls.Addr != "" {
				uaddr, err = net.ResolveUnixAddr(nett, ls.Addr)
				if err != nil {
					return
				}
			}
			basictest = sd.IsUNIXListener(uaddr)
		}

		var filetests []sd.FileTest
		filetests = append(filetests, basictest)
		filetests = append(filetests, ls.ExtraFileTests...)

		ln, name, err = sd.InheritNamedListener(name, filetests...)
		if err != nil {
			return
		}

		if ln == nil {
			if ls.InheritOnly {
				err = ErrNoListener
				return // TODO
			}

			// make a fresh listener
			var new net.Listener
			switch nett {
			case "tcp", "tcp4", "tcp6":
				new, err = net.ListenTCP(nett, taddr)
				if err != nil {
					return
				}
			case "unix", "unixpacket":
				new, err = net.ListenUnix(nett, uaddr)
				if err != nil {
					return
				}
			}
			err = sd.Export(name, new)
			if err != nil {
				new.Close()
				return
			}
			ln = new
		}

		ls.ListenerFdName = name
		if ls.PrepareListener != nil {
			ln = ls.PrepareListener(ln)
		}
		if ls.TLSConfig != nil {
			ln = tls.NewListener(ln, ls.TLSConfig)
			//srv.description = "HTTPS - " + srv.Addr
		}
		listeners = append(listeners, ln)
	}
	return
}
