package gonesrv

import (
	"crypto/tls"
	"github.com/One-com/gone/sd"
	"github.com/One-com/gone/http/graceful"
	"net"
	"fmt"
	"errors"
)

var ErrNoListener = errors.New("No Listener")

// HTTP server implementing the newstyled Server interface
// If ErrorLog is set, errors will be logged to it.
type Server struct {
	*graceful.Server

	description string
	
	// The listener this server has picked to listen on.
	listener     net.Listener

	// ListenerFdName can be set to pick a named file descriptor as
	// Listener via LISTEN_FDNAMES
	// It is updated to contain the name of the chosen file descriptor
	// - if any
	ListenerFdName       string

	ExtraFileTests []sd.FileTest

	// InheritOnly set to true requires the Listener to be inherited via
	// the environment and there will not be created a fresh Listener.
	// Setting InheritOnly true will also disable port 80 as default port
	// and let the Serve listen on any inherited TCP socket it gets
	InheritOnly bool

	// PrepareListener provides a callback to do last minute modifications of
	// the chosen listener. (like wrapping it in something else)
	// It will be called as a callback with the listener chosen before it's set.
	// The returned listener is set instead - wrapped in any TLS if
	// there's a TLSConfig set.
	PrepareListener func (net.Listener) (net.Listener)
}

func (srv *Server) Serve() (err error) {
	err = srv.Server.Serve(srv.listener)
	return
}

func (srv *Server) Description() string {
	return fmt.Sprintf("%s socket(%s)", srv.description, srv.ListenerFdName)
}

// Pick an already open listener FD or create one.
func (srv *Server) Listen() (err error) {
	saddr := srv.Addr
	if saddr == "" && ! srv.InheritOnly {
		saddr = ":http"
	}

	name := srv.ListenerFdName
	var ln net.Listener
	var addr *net.TCPAddr

	if saddr != "" {
		addr, err = net.ResolveTCPAddr("tcp", saddr)
		if err != nil {
			return
		}
	}

	var filetests []sd.FileTest
	filetests = append(filetests, sd.IsTCPListener(addr))
	filetests = append(filetests, srv.ExtraFileTests...)
	
	ln, name, err = sd.InheritNamedListener(name, filetests...)
	if err != nil {
		return
	}

	if ln == nil {
		if srv.InheritOnly {
			err = ErrNoListener
			return
		} else {
			// make a fresh listener
			var tl *net.TCPListener
			tl, err = net.ListenTCP("tcp", addr)
			if err != nil {
				return
			}
			err = sd.Export(name, tl)
			if err != nil {
				return
			}
			ln = tl
		}
	}
	
	srv.ListenerFdName = name
	srv.setListener(ln)
	return
}

// Potentially wrap the listener in any server TLS config.
func (srv *Server) setListener(lin net.Listener) {
	var lout net.Listener
	if srv.PrepareListener != nil {
		lin = srv.PrepareListener(lin)
	}
	if srv.TLSConfig != nil {
		lout = tls.NewListener(lin, srv.TLSConfig)
		srv.description = "HTTPS - " + srv.Addr
	} else {
		lout = lin
		srv.description = "HTTP - " + srv.Addr
	}
	srv.listener = lout
}
