package daemon

import (
	"fmt"
	"errors"
	"context"
	"sync"
	"github.com/One-com/gone/sd"

)

// Server is the interface of objects daemon.Run() will manage.
// These objects will be single-use only with a lifetime:
// Listen, Serve, Shutdown, - and possibly Close() if Shutdown exits non-nil
type Server interface {
	// Listen will
	Listen() error
	// Serve will start serving until the context is canceled at which
	// point it will stop generating new activity and exit.
	Serve(context.Context) error
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
			err = s.Listen()
			if err != nil {
				// TODO - what to do about already successful listeners.
				return
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
		controlServer(actionShutdown, ctx, &err, dwg, s)
	}

	dwg.Wait()
	return

}

func (se serverEnsemble) Close() (err error) {
	dwg := new(sync.WaitGroup)

	for i := range se.servers {
		s := se.servers[len(se.servers)-i-1] // reverse order
		controlServer(actionClose, nil, &err, dwg, s)
	}

	dwg.Wait()
	return
}


// serve implements the main logic of handling the list of running servers.
// First let then Listen().
// if no error, then start all by calling Serve()
// Then call the ready callback and wait for them to exit
func serve(ctx context.Context, server Server) (err error) {

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
	actionServe = "Serve"
	actionShutdown = "Shutdown"
	actionClose = "Close"
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
			err = s.Shutdown(ctx)
		case actionClose:
			err = s.Close()
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
