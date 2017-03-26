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

// serve implements the main logic of handling the list of running servers.
// First let then Listen().
// if no error, then start all by calling Serve()
// Then call the ready callback and wait for them to exit
func serve(ctx context.Context, servers []Server, ready_cb func() error) (err error) {

	if servers == nil || len(servers) == 0 {
		return errors.New("No servers")
	}

	// Prepare for run by letting each server Listen
	for _, s := range servers {
		if s != nil {
			err = s.Listen()
			if err != nil {
				// TODO - what to do about already successful listeners.
				return
			}
		}
	}

	// We have all the network files we need now
	// Make the sd state close the rest
	sd.Cleanup()

	dwg := new(sync.WaitGroup)

	// Now start
	for _, s := range servers {
		startServer(ctx, dwg, s)
	}
	
	// Notify that we are running
	if ready_cb != nil {
		nerr := ready_cb()
		if nerr != nil {
			Log(LvlERROR, fmt.Sprintf("Ready callback error: %s", nerr.Error()))
		}
	}

	// Wait for all servers to exit
	dwg.Wait()

	// This serve is finished. Allow a new to start with the FDs we used.
	sd.Reset()

	return
}

// Launch a specific server - does not block
func startServer(ctx context.Context, done *sync.WaitGroup, s Server) {

	done.Add(1)

	// Start the service it self
	go func() {
		defer done.Done()

		var description string
		description = "NOGET"
		//if ds, ok := s.(Descripter); ok {
		//	description = ds.Description()
		//}

		Log(LvlINFO, fmt.Sprintf("Starting (%s)", description))

		// We sit here until "something" provokes Serve() to exit
		// We have to log the error returned from the individual Serve()
		err := s.Serve(ctx)
		if err != nil {
			Log(LvlERROR, fmt.Sprintf("Server (%s) error: %s",
				description,
				err.Error()))
		} else {
			Log(LvlINFO, fmt.Sprintf("Exited (%s)", description))
		}
	}()
}
