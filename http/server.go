package http

import (
	"net"
	"net/http"
	"sync"
	"context"
	"crypto/tls"
)

// Listener is an object which can create a slice of listeners when invoked.
type Listener interface {
	Listen() (listeners []net.Listener, err error)
}

// Server wraps around gone/http/graceful HTTP server implementing gone/the daemon/srv.Server interface
// If ErrorLog is set, errors will be logged to it.
type Server struct {
	*http.Server
	Listeners Listener
	listeners []net.Listener
}

// Listen make the server listen on the listeners returned by the object set in
// in the Listeners attribute. If Listeners is nil, the server will listen on
// the Addr attribute possibly wrapped with TLSConfig
func (s *Server) Listen() (err error) {

	saddr := s.Addr
	var listeners []net.Listener

	spec := s.Listeners
	
	if spec == nil {

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
	
	var wg sync.WaitGroup
	var mu sync.Mutex // protect the err return value

	// First start serving on all the listeners.
	for _, l := range listeners {
		listener := l
		wg.Add(1)
		go func() {
			defer wg.Done()
			lerr := s.Server.Serve(listener)
			mu.Lock()
			if err == nil {
				err = lerr
			}
			mu.Unlock()
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

