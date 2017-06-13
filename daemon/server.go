package daemon

import (
	"context"
	"errors"
	"fmt"
	"github.com/One-com/gone/sd"
	"sync"
)

// Server is the interface of objects daemon.Run() will manage.
// These objects will be single-use only with a lifetime:
// Listen, Serve, Shutdown, - and possibly Close() if Shutdown exits non-nil
type Server interface {
	// Serve will start serving until the context is canceled at which
	// point it will stop generating new activity and exit.
	Serve(context.Context) error
}

// ListeningServer is a Server which wished to have its Listen() method called before Serve()
type ListeningServer interface {
	Server
	// Listen will be called before Serve() is called to allow the server
	// to prepare for serving. It doesn't need to actually do network listening.
	// it's just an opportunity to get a pre-serve call.
	Listen() error
}

// LingeringServer is a Server which potentially has background activity even after Serve() has exited.
// This could be connections still open and processing request, even though the listeners have closed.
type LingeringServer interface {
	Server
	// Shutdown will wait for all activity to stop until the context
	// is canceled at which point it will exit with an error if activity has
	// not stopped.
	Shutdown(context.Context) error
	// Close() will force all activity to stop.
	Close() error
}

// descriptor - A Server implementing the descriptor interface will use that description in any logging
type descriptor interface {
	Description() string
}

type serverEnsemble struct {
	servers []Server
	readycb func() error
}

var errNoServers = errors.New("No servers")

func (se serverEnsemble) Listen() (err error) {

	if len(se.servers) == 0 {
		return errNoServers
	}

	// Prepare for run by letting each server Listen
	for _, s := range se.servers {
		if s != nil {
			if ls, ok := s.(ListeningServer); ok {
				err = ls.Listen()
				if err != nil {
					// TODO - what to do about already successful listeners.
					return
				}
			}
		}
	}
	return
}

func (se serverEnsemble) Serve(ctx context.Context) (err error) {

	dwg := new(sync.WaitGroup)

	// Now start
	for _, s := range se.servers {
		controlServer(actionServe, ctx, &err, dwg, s)
	}

	// Notify that we are running
	if se.readycb != nil {
		nerr := se.readycb()
		if nerr != nil {
			Log(LvlERROR, fmt.Sprintf("Ready callback error: %s", nerr.Error()))
		}
	}

	// Wait for all servers to exit
	dwg.Wait()
	return
}

func (se serverEnsemble) Shutdown(ctx context.Context) (err error) {

	dwg := new(sync.WaitGroup)

	for i := range se.servers {
		s := se.servers[len(se.servers)-i-1] // reverse order
		if ls, ok := s.(LingeringServer); ok {
			controlServer(actionShutdown, ctx, &err, dwg, ls)
		}
	}

	dwg.Wait()
	return

}

func (se serverEnsemble) Close() (err error) {
	dwg := new(sync.WaitGroup)

	for i := range se.servers {
		s := se.servers[len(se.servers)-i-1] // reverse order
		if ls, ok := s.(LingeringServer); ok {
			controlServer(actionClose, nil, &err, dwg, ls)
		}
	}

	dwg.Wait()
	return
}

// serve implements the main logic of handling the list of running servers.
// First let then Listen().
// if no error, then start all by calling Serve()
// Then call the ready callback and wait for them to exit
func serve(ctx context.Context, server ListeningServer) (err error) {

	if server == nil {
		return errNoServers
	}

	err = server.Listen()
	if err != nil {
		return
	}

	// We have all the network files we need now
	// Make the sd state close the rest
	sd.Cleanup()

	err = server.Serve(ctx)
	if err != nil {
		return
	}

	// This serve is finished. Allow a new to start with the FDs we used.
	sd.Reset()

	return
}

const (
	actionServe    = "Serve"
	actionShutdown = "Shutdown"
	actionClose    = "Close"
)

// Launch a specific server - does not block
func controlServer(action string, ctx context.Context, firsterr *error, done *sync.WaitGroup, s Server) {

	var errmu sync.Mutex

	done.Add(1)

	// Start the service it self
	go func() {
		defer done.Done()

		var description string
		if ds, ok := s.(descriptor); ok {
			description = ds.Description()
		}

		Log(LvlINFO, fmt.Sprintf("%s (%s)", action, description))

		// We sit here until "something" provokes action to exit
		// We have to log the error returned from the individual action
		var err error
		switch action {
		case actionServe:
			err = s.Serve(ctx)
		case actionShutdown:
			if ls, ok := s.(LingeringServer); ok {
				err = ls.(LingeringServer).Shutdown(ctx)
			} else {
				// Unreachable
				Log(LvlCRIT, fmt.Sprintf("Action %s (%s) error: illegal action",
					action, description))
			}
		case actionClose:
			if ls, ok := s.(LingeringServer); ok {
				err = ls.(LingeringServer).Close()
			} else {
				// Unreachable
				Log(LvlCRIT, fmt.Sprintf("Action %s (%s) error: illegal action",
					action, description))
			}
		default:
			Log(LvlCRIT, fmt.Sprintf("Action %s (%s) error: unknown action",
				action, description))
			return
		}
		if err != nil {
			Log(LvlERROR, fmt.Sprintf("%s (%s) error: %s",
				action, description,
				err.Error()))
			errmu.Lock()
			if *firsterr == nil {
				*firsterr = err
			}
			errmu.Unlock()
		} else {
			Log(LvlINFO, fmt.Sprintf("%s exited (%s)", action, description))
		}
	}()
}
