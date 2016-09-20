package srv

import (
	"errors"
	"fmt"
	"github.com/One-com/gone/sd"
	"sync"
)

// Syslog priority levels
const (
	LvlEMERG int = iota // Not to be used by applications.
	LvlALERT
	LvlCRIT
	LvlERROR
	LvlWARN
	LvlNOTICE
	LvlINFO
	LvlDEBUG
)

// A LoggerFunc can be set to make the newstyle library log to a custom log library
type LoggerFunc func(level int, message string)

// SetLogger sets a custom log function.
// The default log function logs to stdlib log.Println()
func (m *MultiServer) SetLogger(f LoggerFunc) {
	m.logmu.Lock()
	defer m.logmu.Unlock()
	m.logger = f
}

// Server is an server started by the master MultiServer, by calling Serve() which is expected to block
// until the server is signaled to stop by an invocation of Shutdown()
//
// Calling Shutdown() should start the shutdown process and return immediately,
// causing Serve() to exit.
// If the server implements Wait() its Serve() method can exit asynchronously before
// shutdown is fully completed. The master server will call Wait() before any restart of the server.
//
// If Serve() returns no error, the master server regards the server as fully finished
// and ready to (maybe) be restarted by another invocation of the master MultiServer
// Serve() method.
type Server interface {
	Serve() error // start serving and block until exit
	Shutdown()    // async req. to shutdown, must not block
}

// Listener - Servers implementing the Listener interface will be called upon to Listen() before being asked to Serve()
type Listener interface {
	Listen() error
}

// Descripter - A Server implementing the Descriptor interface will use that description in any logging
type Descripter interface {
	Description() string
}

// Waiter - implmented on Servers if they expect the caller of Serve() to Wait() for complete shutdown.
// Servers are allowed to exit Serve() before being fully shutdown immediately after Shutdown() by implementing Wait() to
// allow the master MultiServer to wait for full shutdown.
// Servers not implementing Wait() are expected to be fully shutdown and restartable
// once Serve() exists with a non-nil error.
// Calling Wait() on a not running and fully shut down server should be a NOOP.
type Waiter interface {
	Wait()
}

// MultiServer manages a slice of servers implementing the Serve interface (and possibly Listener/Waiter)
// The servers are started by calling Serve() and stopped by calling Shutdown().
type MultiServer struct {
	logmu  sync.RWMutex
	logger LoggerFunc

	mu      sync.Mutex // protects the "running" attr of the master server
	servers []Server
	exited  sync.WaitGroup  // cleared when all servers exited Serve()
	done    *sync.WaitGroup // cleared when all servers shut down. new for each run
	running bool
	wait    bool // let Serve() wait for all servers before exit
	err     error
}

// Log is used by a MultiServer to log internal actions if a LoggerFunc is set.
// You can call this your self if you need to. It's go-routine safe if the provided Log function is.
func (m *MultiServer) Log(level int, msg string) {
	m.logmu.RLock()
	if m.logger != nil {
		m.logger(level, msg)
	}
	m.logmu.RUnlock()
}

// Serve starts the supplied "servers" and waits until signaled to stopped via Shutdown() call.
// Once the servers are successfully started the callback will be called.
// Serve will exit as soon as all servers have recognized the Shutdown().
// Servers can acknowledge Shutdown() without having fully exited if they
// implement the Wait() method. The done channel will be closed when all
// servers have exited and any implementors of Wait() has been waited on.
func (m *MultiServer) Serve(servers []Server, ready_cb func() error) (done chan struct{}, err error) {

	if servers == nil || len(servers) == 0 {
		return nil, errors.New("No servers")
	}

	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return nil, errors.New("Already serving")
	}

	m.servers = servers
	done_chan := make(chan struct{})

	dwg := new(sync.WaitGroup)
	m.done = dwg

	err = m.start() // start the servers - does not block
	if err != nil {
		m.mu.Unlock()
		return
	}

	m.running = true
	m.mu.Unlock()

	// Make something close the done channel when all servers have no more activity
	go func() {
		dwg.Wait()
		close(done_chan)
	}()

	// Notify that we are running
	if ready_cb != nil {
		nerr := ready_cb()
		if nerr != nil {
			m.mu.Lock()
			m.err = nerr
			m.mu.Unlock()
		}
	}

	// Wait for all servers to exit
	m.exited.Wait()

	// This serve is finished. Allow a new to start with the FDs we used.
	sd.Reset()

	m.mu.Lock()
	m.running = false // don't allow us to be called again before now
	m.mu.Unlock()

	return done_chan, m.err
}

// Shutdown send an async signal to the server to exit Serve() by calling Shutdown in the
// individual servers
func (m *MultiServer) Shutdown() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		m.stop()
	}
}

//------------------------ internal ---------------------------------------

func (m *MultiServer) start() (err error) {

	for _, s := range m.servers {
		if s != nil {
			if l, ok := s.(Listener); ok {
				err = l.Listen()
				if err != nil {
					return
				}
			}
		}
	}

	// We have all the network files we need now
	// Make the sd state close the rest
	sd.Cleanup()

	// take an initial token to avoid falling through Wait()
	// before any servers are started
	m.exited.Add(1)

	for _, s := range m.servers {
		if m != nil {
			m.startServer(s) // does not block
		}
	}

	return nil
}

func (m *MultiServer) stop() {
	// Tell all servers to stop
	for _, s := range m.servers {
		if m != nil {
			var description string
			if ds, ok := s.(Descripter); ok {
				description = ds.Description()
			}
			m.Log(LvlDEBUG, fmt.Sprintf("Shutting down (%s)", description))
			s.Shutdown()
		}
	}

	// release our initial token allowing the Wait() procedure
	// when the last protocol server exits
	m.exited.Done()
}

// Launch a specific server - does not block
func (m *MultiServer) startServer(s Server) {

	m.exited.Add(1) // Undone when the Server go-routine exits

	done := m.done
	exited := make(chan struct{})

	done.Add(1) // Undone when the Waiter go-routine exits

	// start a go-routine to wait for a "waiter" efter servers have exited.
	go func() {
		// Wait for server to exit, then decide whether to wait for done.
		<-exited
		// Servers not implementing Wait() are expected to be "done"
		// when they have "exited".
		if waiter, ok := s.(Waiter); ok {
			// fmt.Println("Waiting for single service")
			waiter.Wait()
		}
		//		fmt.Println("Single service done")
		done.Done()
	}()

	// Start the service it self
	go func() {
		defer m.exited.Done()

		var description string
		if ds, ok := s.(Descripter); ok {
			description = ds.Description()
		}

		m.Log(LvlINFO, fmt.Sprintf("Starting (%s)", description))

		// We sit here until "something" provokes Serve() to exit
		// We have to log the error returned from the individual Serve()
		// since the master server err return can 1) not convey multiple errors,
		// and 2) is meant to indicate problems of the master server it self.
		err := s.Serve()
		if err != nil {
			m.mu.Lock()
			m.err = err
			m.mu.Unlock()
			m.Log(LvlERROR, fmt.Sprintf("Server (%s) error: %s",
				description,
				err.Error()))
		} else {
			m.Log(LvlINFO, fmt.Sprintf("Exited (%s)", description))
		}
		close(exited)
	}()
}
