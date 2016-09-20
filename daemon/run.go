package daemon

import (
	"errors"
	"fmt"
	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/sd"
	"os"
	"sync"
	"syscall"
)

// CleanupFunc is a function to call after a srv.Server is fully exited.
// A slice of CleanupFunc will be called after all servers are completely done.
// These can be used to - say - close files.
type CleanupFunc func() error

// ConfigureFunc is a function returning srv.Server to run and the CleanupFuncs to call
// when they have completely shut down.
// The Run() function needs a ConfigureFunc to instantiate the Servers to serve.
type ConfigureFunc func() ([]srv.Server, []CleanupFunc, error)

var (
	srvmu    sync.Mutex
	revision int
	servers  []srv.Server
	cleanups []CleanupFunc

	stopch chan bool // true to do graceful shutdown
	reload chan struct{}
)

var _master *srv.MultiServer

func init() {
	// creat the channel to propate reload events to the reload manager
	reload = make(chan struct{}, 1) // async

	// create the channel which tells the master reload-loop to exit
	stopch = make(chan bool, 1) // async

	_master = &srv.MultiServer{}
}

// SetLogger sets a custom log function.
func SetLogger(f srv.LoggerFunc) {
	_master.SetLogger(f)
}

// Log calls the custom log function set - if any
func Log(level int, msg string) {
	_master.Log(level, msg)
}

type runcfg struct {
	instantiate    ConfigureFunc
	syncReload     bool
	readyCallbacks []func() error
}

// RunOption change the behaviour of Run()
type RunOption func(*runcfg)

// InstantiateServers gives Run() a ConfigureFunc. This is the only mandatory RunOption
func InstantiateServers(f ConfigureFunc) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.instantiate = f
	})
}

// SyncReload makes Run() Wait() for all servers before starting the next generation om Reload()
func SyncReload() RunOption {
	return RunOption(func(rc *runcfg) {
		rc.syncReload = true
	})
}

// ReadyCallback sets a function to be called when all servers have started without error
func ReadyCallback(f func() error) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.readyCallbacks = append(rc.readyCallbacks, f)
	})
}

// SignalParentOnReady sets a ReadyCallback which signals the parent process to terminate.
func SignalParentOnReady() RunOption {
	return RunOption(func(rc *runcfg) {
		rc.readyCallbacks = append(rc.readyCallbacks, sd.SignalParentTermination)
	})
}

// SdNotifyOnReady makes Run() notify systemd with STATUS=READY when all servers have started.
// If mainpid is true, the MAINPID of the current process is also notified.
func SdNotifyOnReady(mainpid bool, status string) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.readyCallbacks = append(rc.readyCallbacks, func() error {
			var msg [3]string
			c := 0
			msg[c] = "READY=1"
			c++
			if mainpid {
				pid := os.Getpid()
				msg[c] = fmt.Sprintf("MAINPID=%d", pid)
				c++
			}
			if status != "" {
				msg[c] = fmt.Sprintf("STATUS=%s", status)
				c++
			}
			err := sd.Notify(0, msg[0:c]...)
			if err == sd.ErrSdNotifyNoSocket {
				Log(srv.LvlWARN, "No systemd notify socket")
				return nil
			}
			return err
		})

	})
}

// Run takes a set of RunOptions. The only mandatory option is InstantiateServers.
// The servers will be managed and via Serve() and can be controlled with various functions,
// like Reload() and Exit()
// On Reload() Run() will try to instantiate a new set of servers and if successful will
// replace the current running servers with the new set, using the gone/sd package to
// re-create sockets without closing TCP connections.
func Run(opts ...RunOption) (err error) {
	cfg := &runcfg{readyCallbacks: make([]func() error, 0)}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.instantiate == nil {
		return errors.New("Don't know how to configure servers")
	}

	readyCallback := func() error {
		var err error
		for _, f := range cfg.readyCallbacks {
			err = f()
		}
		return err
	}

	var first_mu sync.Mutex
	first_config_load_done := make(chan struct{})

	// reload handler
	go func() {
		for {
			// Wait for reload signal
			<-reload
			s, c, err := cfg.instantiate()
			if err == nil {
				// Replace the server to run with a new slice.
				srvmu.Lock()
				servers = s
				cleanups = c
				revision++
				srvmu.Unlock()
				_master.Shutdown() // noop if not started
			} else {
				_master.Log(srv.LvlCRIT, "Error setting up services (not reloading)")
			}
			first_mu.Lock()
			if first_config_load_done != nil {
				close(first_config_load_done)
			}
			first_mu.Unlock()
		}
	}()

	// Load the initial config
	reload <- struct{}{}
	<-first_config_load_done
	first_mu.Lock()
	first_config_load_done = nil
	first_mu.Unlock()

	var graceful_exit bool
	var done chan struct{}

MainLoop:
	for {
		srvmu.Lock()
		if servers == nil {
			srvmu.Unlock()
			return errors.New("No Config")
		}
		running_servers := servers
		running_revision := revision
		running_cleanups := cleanups
		srvmu.Unlock()

		// Start serving the currently configured servers
		if done, err = _master.Serve(running_servers, readyCallback); err != nil {
			return
		}

		/* Should we exit? */
		select {
		case graceful_exit = <-stopch:
			break MainLoop
		default:
			if cfg.syncReload {
				recordShutdown(running_revision, running_cleanups, done)
			} else {
				go recordShutdown(running_revision, running_cleanups, done)
			}
		}
	}
	_master.Log(srv.LvlNOTICE, "Exit mainloop")
	if graceful_exit {
		srvmu.Lock()
		_master.Log(srv.LvlNOTICE, "Waiting for graceful shutdown")
		recordShutdown(revision, cleanups, done)
		srvmu.Unlock()
	}
	return
}

func recordShutdown(rev int, cleanups []CleanupFunc, done chan struct{}) {
	<-done
	for _, f := range cleanups {
		e := f()
		if e != nil {
			_master.Log(srv.LvlWARN, fmt.Sprintf("Cleanup failed: %s", e.Error()))
		}
	}
	_master.Log(srv.LvlNOTICE, fmt.Sprintf("All servers (rev=%d) shutdown", rev))
}

// Reload tells Run() to instatiate new servers and continue serving with them.
func Reload() {
	// don't wait if a reload is in progress
	select {
	case reload <- struct{}{}:
	default:
		_master.Log(srv.LvlNOTICE, "Reload already pending")
	}
}

// Exit tells Run() to exit. If graceful is true, Run() will wait for all servers to nicely cleanup.
func Exit(graceful bool) {
	select {
	case stopch <- graceful: // buffered by 1 exit operation at a time
		_master.Shutdown()
	default:
		_master.Log(srv.LvlNOTICE, "Main loop already waiting on exit")
	}
}

// ReplaceProcess spawns a new version of the program.
// sig is the UNIX signal to send to terminate the parent once we're up and running
func ReplaceProcess(sig syscall.Signal) (int, error) {
	return sd.ReplaceProcess(sig)
}
