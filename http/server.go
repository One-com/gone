package http

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"

	"github.com/One-com/gone/netutil"
)

// Server wraps around gone/http/graceful HTTP server implementing gone/the daemon/srv.Server interface
// If ErrorLog is set, errors will be logged to it.
type Server struct {
	*http.Server
	// The object providing the listeners for this server
	Listeners netutil.StreamListener
	listeners []net.Listener
	// Optionally set a name to by used in logging
	Name string
}

// Listen make the server listen on the listeners returned by the object set in
// in the Listeners attribute. If Listeners is nil, the server will listen on
// the Addr attribute possibly wrapped with TLSConfig
func (s *Server) Listen() (err error) {

	saddr := s.Addr
	var listeners []net.Listener

	if s.Listeners == nil {

		if saddr == "" {
			if s.TLSConfig == nil {
				saddr = ":http"
			} else {
				saddr = ":https"
			}
		}
		var ln net.Listener
		ln, err = net.Listen("tcp", saddr)
		if err != nil {
			return

		}
		if s.TLSConfig != nil {
			ln = tls.NewListener(ln, s.TLSConfig)
		}
		listeners = []net.Listener{ln}
	} else {
		listeners, err = s.Listeners.Listen()
	}

	s.listeners = listeners

	return
}

// Serve will call net/http.Server.Serve() on all listeners, closing the listeners
// when the context is canceled to make Serve() exit.
// This method exits when all underlying Serve() calls have exited.
func (s *Server) Serve(ctx context.Context) (err error) {

	listeners := s.listeners
	if len(listeners) == 0 {
		return errors.New("No HTTP listeners for " + s.Name)
	}

	var wg sync.WaitGroup
	var mu sync.Mutex // protect the err return value

	// First start serving on all the listeners.
	for _, l := range listeners {
		listener := l
		wg.Add(1)
		go func() {
			defer wg.Done()
			lerr := s.Server.Serve(listener)
			if lerr != nil {
				// This is nuts... Hey Go. You can do better
				// https://github.com/golang/go/issues/4373
				switch et := lerr.(type) {
				case *net.OpError:
					switch et.Op {
					case "accept":
						// TCP servers which stop by closing the listener socket
						// ignore
						lerr = nil
					}
				default:
					// let other errors pass
				}
				// end hack
				mu.Lock()
				if err == nil {
					err = lerr
				}
				mu.Unlock()
			}
		}()
	}

	// then make something close the listeners when context is canceled
	exit := make(chan struct{}) // ...and stop it when all are closed
	go func() {
		select {
		case <-ctx.Done():
			for _, l := range listeners {
				l.Close()
			}
		case <-exit:
		}
	}()
	// Wait for the exit of all serve calls
	wg.Wait()
	close(exit)
	return
}

// Description implements a default textual description for a Server objects
// describing what it's up to.
func (s *Server) Description() string {

	var listeners string
	for i, l := range s.listeners {
		listeners += l.Addr().Network() + "/" + l.Addr().String()
		if i != len(s.listeners)-1 {
			listeners += " "
		}
	}
	desc := fmt.Sprintf("%s %s", s.Name, listeners)

	return desc
}
