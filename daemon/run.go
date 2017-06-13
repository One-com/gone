package daemon

import (
	"context"
	"errors"
	"fmt"
	"github.com/One-com/gone/daemon/ctrl"
	"github.com/One-com/gone/daemon/srv"
	"github.com/One-com/gone/sd"
	"os"
	"sync"
	"syscall"
	"time"
)

// CleanupFunc is a function to call after a srv.Server is fully exited.
// A slice of CleanupFunc will be called after all servers are completely done.
// These can be used to - say - close files.
type CleanupFunc func() error

// ConfigureFunc is a function returning srv.Server to run and the CleanupFuncs to call
// when they have completely shut down.
// This function is deprecated. Use ConfigFunc instead and implement your servers as
// daemon.Server instead of srv.Server - if possible.
// The Run() function needs a ConfigureFunc to instantiate the Servers to serve.
type ConfigureFunc func() ([]srv.Server, []CleanupFunc, error)

// ConfigFunc is a function returning Servers and Cleanups for Run() to run.
// Run() will call this every time it needs to configure it self on start and on reload.
// Run() will need either a ConfigFunc or the (deprecated) ConfigureFunc.
type ConfigFunc func() ([]Server, []CleanupFunc, error)

var (
	// Permanent channels. Never closed
	// If Run() is called again after exit there could be left over events on these.
	// Normally Run() shouldn't be called again, but it can be useful in tests.
	stopch   chan bool          // true to do graceful shutdown
	tostopch chan time.Duration // stop gracefully with timeout
	reload   chan struct{}      // reload the daemon config
)

func init() {
	// create the channel to propagate reload events to the reload manager
	reload = make(chan struct{}, 1) // 1 to take pending into account

	// create the channel which tells the master reload-loop to exit
	stopch = make(chan bool, 1) // 1 to take pending into account
	tostopch = make(chan time.Duration, 1)
}

type runcfg struct {
	legacycfgfunc  ConfigureFunc
	cfgfunc        ConfigFunc
	syncReload     bool
	readyCallbacks []func() error
	ctrlSockPath   string
	ctrlSockName   string
	timeout        time.Duration
}

// RunOption change the behaviour of Run()
type RunOption func(*runcfg)

// Configurator gives Run() a ConfigFunc. This is the only mandatory RunOption
// (except you when you use the legacy InstantiateServers() option and supply a
// deprecated "ConfigureFunc" instead)
func Configurator(f ConfigFunc) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.cfgfunc = f
	})
}

// InstantiateServers gives Run() a ConfigureFunc. This is the only mandatory RunOption
func InstantiateServers(f ConfigureFunc) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.legacycfgfunc = f
	})
}

// ControlSocket is an option to provude a systemd socket name and/or a path for the
// daemon control socket. Providing "","" disables the control socket.
func ControlSocket(name, path string) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.ctrlSockPath = path
		rc.ctrlSockName = name
	})
}

// ShutdownTimeout is an option to Run() to control the default timeout value for
// graceful shutdowns. This is used when OS signals or similar crude interface triggers
// a graceful shutdown. Using a control socket you can provide the timeout duration at
// shutdown time.
func ShutdownTimeout(to time.Duration) RunOption {
	return RunOption(func(rc *runcfg) {
		rc.timeout = to
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
				Log(LvlWARN, "No systemd notify socket")
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

	var (
		srvmu     sync.Mutex
		revision  int
		configErr error
		servers   []Server
		cleanups  []CleanupFunc

		nextContext context.Context
		nextCancel  context.CancelFunc
	)

	cfg := &runcfg{readyCallbacks: make([]func() error, 0)}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.cfgfunc == nil && cfg.legacycfgfunc == nil {
		return errors.New("Don't know how to configure servers")
	}

	readyCallback := func() error {
		var err error
		for _, f := range cfg.readyCallbacks {
			e := f()
			if err == nil {
				err = e
			}
		}
		return err
	}

	var exit bool                     // set true when Run() should break the main loop
	var gracefulExit bool             // whether exit of Run() should wait for clean shutdown
	var shutdownTimeout time.Duration // how long to wait for last generation servers to be completely done

	if cfg.timeout != 0 {
		shutdownTimeout = cfg.timeout
	}

	// We cannot serve the first run before the Event handler tells us configuration is done
	//var first_mu sync.Mutex
	firstConfigLoadDone := make(chan struct{})

	// Event handler
	// Even if gone/signals or other code serialized the signals, we don't know how they are wired up and
	// they become async events to the daemon MainLoop again.
	// We need to serialize any event affecting
	// shutdown/restart here again to avoid Exit events arriving after a reload is in effect,
	// but before the new Servers are running. That would loose the exit signal.
	eventch := make(chan struct{})
	go func() {
		var firstConfigDoneOnce sync.Once
	EVENTLOOP:
		for {
			select {
			case timeout := <-tostopch:
				shutdownTimeout = timeout
				gracefulExit = true
				exit = true
				nextCancel()
			case gracefulExit = <-stopch:
				exit = true
				nextCancel()
			// Wait for reload signal
			case <-reload:
				var err error
				var newServers []Server
				var newCleanups []CleanupFunc
				// prefer non-legacy servers
				if cfg.cfgfunc != nil {
					newServers, newCleanups, err = cfg.cfgfunc()
				} else {
					// Wrap all legacy servers in compatibility wrapper
					s, c, e := cfg.legacycfgfunc()
					for _, l := range s {
						newServers = append(newServers, &wrapper{Server: l})
					}
					newCleanups = c
					err = e
				}
				if err == nil {
					// Replace the server to run with a new set of server objects
					srvmu.Lock()
					oldCancel := nextCancel
					nextContext, nextCancel = context.WithCancel(context.Background())
					servers = newServers
					cleanups = newCleanups
					revision++
					srvmu.Unlock()
					// ready to replace, now cancel old runContext and see what happens
					// when serve() exists
					if oldCancel != nil {
						oldCancel()
					}
				} else {
					// configuration failed.
					srvmu.Lock()
					configErr = err
					srvmu.Unlock()
					Log(LvlCRIT, fmt.Sprintf("Daemon reload: %s", configErr.Error()))
				}
				// Main loop might be waiting for the first config. Notify it's done.
				firstConfigDoneOnce.Do(func() { close(firstConfigLoadDone) })

			case <-eventch:
				nextCancel()
				break EVENTLOOP
			}
		}
	}()

	// Load the initial config by asking event manager to load it
	reload <- struct{}{}
	// Wait here until event manager is ready with first config
	<-firstConfigLoadDone

MainLoop:
	for {
		srvmu.Lock()
		// TODO: maybe not refuse to run with no servers?
		if servers == nil {
			if configErr == nil {
				err = errors.New("No Servers")
			} else {
				err = configErr
			}
			srvmu.Unlock()
			return
		}

		// Set up any control socket
		if cfg.ctrlSockName != "" || cfg.ctrlSockPath != "" {
			cs := &ctrl.Server{
				Addr:           cfg.ctrlSockPath,
				ListenerFdName: cfg.ctrlSockName,
				HelpCommand:    "?",
				QuitCommand:    "q",
				Logger:         Log,
			}
			csdone := make(chan struct{})
			err = cs.Listen()
			if err != nil {
				Log(LvlCRIT, fmt.Sprintf("Failed listen on control socket: %s", err.Error()))
			}
			go func() {
				err = cs.Serve()
				if err != nil {
					Log(LvlCRIT, fmt.Sprintf("Control socket exited with error: %s", err.Error()))
				}
				close(csdone)
			}()

			// Append a last cleanup which closes the control socket:
			cleanups = append(cleanups, func() error {
				// Stop control socket
				if cs != nil {
					cs.Shutdown()
					<-csdone
					Log(LvlNOTICE, "Control socket shut down")
				} else {
					Log(LvlNOTICE, "No control socket")
				}
				return nil
			})
		}

		runningServer := serverEnsemble{servers, readyCallback}
		runningRevision := revision
		runningCleanups := cleanups

		runningContext := nextContext

		srvmu.Unlock()

		// Start serving the currently configured servers
		if err = serve(runningContext, runningServer); err != nil {
			return
		}

		/* Should we exit? */
		if exit {
			break MainLoop
		}

		if cfg.syncReload {
			recordShutdown(runningRevision, runningServer, runningCleanups, 0)
		} else {
			go recordShutdown(runningRevision, runningServer, runningCleanups, 0)
		}
	} // end MainLoop

	Log(LvlNOTICE, "Exit mainloop")
	if gracefulExit {
		srvmu.Lock()
		Log(LvlNOTICE, "Waiting for graceful shutdown")
		recordShutdown(revision, serverEnsemble{servers, nil}, cleanups, shutdownTimeout)
		srvmu.Unlock()
	}
	close(eventch)
	return
}

func recordShutdown(rev int, server LingeringServer, cleanups []CleanupFunc, timeout time.Duration) {

	var (
		ctx    context.Context
		cancel context.CancelFunc
	)
	if timeout == 0 {
		ctx = context.Background()
	} else {
		ctx, cancel = context.WithTimeout(context.Background(), timeout)
		defer cancel()
	}

	e := server.Shutdown(ctx)
	if e != nil {
		Log(LvlERROR, "Forcefully closing...")
		e = server.Close()
		if e != nil {
			Log(LvlCRIT, "Forcefully closing failed")
		}
	}

	// All servers done - either voluntarily or the hard way.
	// Run cleanups
	for _, f := range cleanups {
		if f == nil {
			Log(LvlWARN, "Not running nil cleanup")
			continue
		}
		e := f()
		if e != nil {
			Log(LvlWARN, fmt.Sprintf("Cleanup failed: %s", e.Error()))
		}
	}
	Log(LvlNOTICE, fmt.Sprintf("All servers (rev=%d) shutdown", rev))
}

// Reload tells Run() to instatiate new servers and continue serving with them.
func Reload() {
	// don't wait if a reload is in progress
	select {
	case reload <- struct{}{}:
	default:
		Log(LvlNOTICE, "Reload already pending")
	}
}

// Exit tells Run() to exit. If graceful is true, Run() will wait for all servers to nicely cleanup.
func Exit(graceful bool) {
	select {
	case stopch <- graceful: // buffered by 1 exit operation at a time
	default:
		Log(LvlNOTICE, "Main loop already waiting on exit")
	}
}

// ExitGracefulWithTimeout is like Exit(true), but has a timeout to fall back to
// the effect of Exit(false)
func ExitGracefulWithTimeout(to time.Duration) {
	select {
	case tostopch <- to: // buffered by 1 exit operation at a time
	default:
		Log(LvlNOTICE, "Main loop already waiting on exit")
	}
}

// ReplaceProcess spawns a new version of the program.
// sig is the UNIX signal to send to terminate the parent once we're up and running
func ReplaceProcess(sig syscall.Signal) (int, error) {
	return sd.ReplaceProcess(sig)
}
