package graceful

import (
	"errors"
	"net"
	"net/http"
	"sync"
	"time"
)

// *heavily* inspired from
// https://github.com/tylerb/graceful
// But without the os.Signal stuff - KISS

// Server implements an extended http.Server with a Shutdown() method which enables it
// to be shut down gracefully, by first stopping accepting and disabling keepalives
// and then wait for all connections to have terminated.
// A Timeout can be set to finally kill the last outstanding connections forcefully.
type Server struct {
	*http.Server

	// Timeout is the duration to allow outstanding requests to survive
	// before forcefully terminating them.
	// A timeout of 0 will make the server wait forever for connections to close
	// by them selves. (as per github.com/tylerb/graceful doc.)
	Timeout time.Duration

	// ConnState specifies an optional callback function that is
	// called when a client connection changes state. This is a proxy
	// to the underlying http.Server's ConnState, and the original
	// must not be set directly.
	ConnState func(net.Conn, http.ConnState)

	// shutdown signals the Server to stop serving connections,
	// and the server to start shutdown proceedure
	// (by reading its first and only value)
	shutdown chan bool

	// Shutdown manager will close this one when shutdown is complete
	done chan struct{}

	// SyncShutdown makes the server not exit immediately on Shutdown signal.
	// But wait for keep-alive connections to finish.
	// If not setting SyncShutdown true, Call Wait() to wait for shutdown to finish
	SyncShutdown bool

	// Ensure only one Serve() call can be active at at time.
	// When not using SyncShutdown however, a connectionManager/shutdownHandler
	// can be finishing off an old set of connections in the background
	runlock  sync.Mutex
	quitting bool
	running  bool
	killed   int
}

// NotReadyError is returned from a Serve() call if the server is already running, or during shutdown.
type NotReadyError struct {
	Err error
	// Quitting is true when a shutdown is in progress
	Quitting bool
}

func (e *NotReadyError) Error() string {
	return e.Err.Error()
}

func (srv *Server) ConnectionsKilled() int {
	srv.runlock.Lock()
	defer srv.runlock.Unlock()
	return srv.killed
}

// Serve is equivalent to net/http.Server.Serve() with graceful shutdown enabled.
func (srv *Server) Serve(listener net.Listener) error {

	srv.runlock.Lock()
	if srv.quitting {
		srv.runlock.Unlock()
		return &NotReadyError{
			Err:      errors.New("Shutdown in progress"),
			Quitting: true,
		}
	}
	if srv.running {
		srv.runlock.Unlock()
		return &NotReadyError{
			Err: errors.New("Already running"),
		}
	}

	// Take responsibility of the server object and start serving
	srv.running = true
	srv.quitting = false

	// A channel to ask the server to shutdown
	srv.shutdown = make(chan bool)
	// A channel for clients to wait for shutdown complete
	srv.done = make(chan struct{})
	srv.killed = 0
	srv.runlock.Unlock()

	// Track connection state
	add := make(chan net.Conn)
	remove := make(chan net.Conn)

	srv.Server.ConnState = func(conn net.Conn, state http.ConnState) {
		switch state {
		case http.StateNew:
			add <- conn
		case http.StateClosed, http.StateHijacked:
			remove <- conn
		}

		if srv.ConnState != nil {
			srv.ConnState(conn, state)
		}
	}

	// Setup up communication between the
	// connection manager and the shutdown manager

	// To signal the connection mananger to stop with a response channel to be
	// replied on when done.
	stop := make(chan chan int)
	// To signal the connection manager it's time to kill any outstanding connections
	// (often keep-alive)
	kill := make(chan struct{})

	// To tell the shutdown manager it's actions have had effect here.
	exited := make(chan struct{})

	go srv.manageConnections(add, remove, stop, kill)

	// Listen for shutdown events and manage the shutdown procedure
	go srv.handleShutdown(listener, stop, kill, exited)

	// Serve with graceful listener.
	// Execution blocks here until listener.Close() is called, above.
	err := srv.Server.Serve(listener)
	// Filter out trivial errors from accept when listener is closed.
	if err != nil {
		// This is nuts... Hey Go. You can do better
		// https://github.com/golang/go/issues/4373
		switch et := err.(type) {
		case *net.OpError:
			switch et.Op {
			case "accept":
				// TCP servers which stop by closing the listener socket
				// ignore
				err = nil
			}
		default:
			// let other errors pass
		}
	}

	// Go into shutdown mode.
	srv.runlock.Lock()
	srv.quitting = true
	srv.runlock.Unlock()

	// Ensure shutdown is activated and record whether it was done by a proper call to shutdown
	// Read the shutdown channel to see if we trigger shutdown here
	// If a proper Shutdown() was requested, we will read false
	extraordinary_exit := <-srv.shutdown
	if extraordinary_exit {
		if srv.ErrorLog != nil {
			srv.ErrorLog.Printf("Server exited without requested: %s", err)
		}
	}

	close(exited) // don't let the shutdown manager run to end unless we've gotten here.

	// HTTP server exited... decide whether to wait for still running connections here
	if srv.SyncShutdown {
		<-srv.done
	}

	return err
}

// ShutdownOK stops the server from accepting new requets and begins shutting down.
// It returns true if this call ended up being the one triggering a shutdown
func (srv *Server) ShutdownOK() bool {
	srv.runlock.Lock()
	defer srv.runlock.Unlock()
	if !srv.running {
		return false
	}
	return <-srv.shutdown
}

// Shutdown signales the server asynchronously to start shutdown process
// It only has effect the first time it's called.
func (srv *Server) Shutdown() {
	srv.runlock.Lock()
	defer srv.runlock.Unlock()
	if !srv.running {
		return
	}
	<-srv.shutdown
}

// Wait for server shutdown to finish (having killed any remaning keep-alive connections)
func (srv *Server) Wait() {
	srv.runlock.Lock()
	done := srv.done
	running := srv.running
	srv.runlock.Unlock()

	if !running {
		return
	}
	<-done
}

func (srv *Server) handleShutdown(listener net.Listener, stop chan chan int, kill, exited chan struct{}) {
	// Send a single object on the stop channel.
	// The first client to read this will unblock this go-routine and trigger
	// a shutdown.
	// All subsequent clients will read a closed channel == false
	srv.shutdown <- true
	close(srv.shutdown)

	// START SHUTDOWN PROCEDURE:
	// fmt.Println("Shutdown initiated")
	// Cause shutdown
	srv.SetKeepAlivesEnabled(false)
	// Stop accepting and thereby stop the flow of add events to the conn. man.
	err := listener.Close()
	if err != nil {
		if srv.ErrorLog != nil {
			srv.ErrorLog.Printf("Error closing listener: %s", err)
		}
	}

	// Wait for effect
	<-exited

	// Request done notification
	// send the done channel to all conns listening for a stop event
	// for them to reply on
	done := make(chan int)
	var killed int
	stop <- done // Tell the conn manager we want to know when its finished

	// Wait for conn manager and handle timeout
	if srv.Timeout > 0 {
		select {
		case killed = <-done:
			// Fine, no need to kill connections.
		case <-time.After(srv.Timeout):
			kill <- struct{}{}
			// wait for reply
			killed = <-done
		}
	} else {
		killed = <-done // Wait for all connection to be shut down
	}

	// cleanup after running
	srv.runlock.Lock()
	srv.killed = killed
	srv.quitting = false
	srv.running = false
	close(srv.done) // server is ready for reuse.
	srv.runlock.Unlock()
}

// Manage open connections
// stop makes the manager exit once connections have dried out them selves (since a closed listener
// will no longer generate add events)
// kill kills off the remaning connections.
func (srv *Server) manageConnections(add, remove chan net.Conn, stop chan chan int, kill chan struct{}) {
	var done chan int
	var killed int
	connections := make(map[net.Conn]struct{}, 10)
	for {
		select {
		case conn := <-add:
			connections[conn] = struct{}{} // remember this conn is alive
		case conn := <-remove:
			delete(connections, conn) // this conn dies, forget it
			// if done is set connections will slowly terminate one
			// after one by them selves assuming the listener has been closed,
			// so Accept() will no longer generate events on the add channel.
			// If done channel is alive and we reached the goal
			if done != nil && len(connections) == 0 {
				done <- killed
				return
			}
		case done = <-stop: // take the done channel we get
			if len(connections) == 0 { // if we were idle, we just return
				done <- killed
				return
			}
		case <-kill: // don't care for the stop reply any more. Now we're killing.
			for k := range connections {
				_ = k.Close() // nothing to do here if it errors
				killed++
			}
			// We have to let the conns remove them selves from here.
			// Else we'll get remove events after we've considered us done.
		}
	}
}
