package daemon

import (
	"context"
	"fmt"
	"time"

	"github.com/One-com/gone/daemon/srv"
)

// wrapper makes srv.Server behave like Server - as close as possible
// srv.Server does not support forced close
// enabling running srv.Server objects in the new
type wrapper struct {
	srv.Server
	done   chan struct{}
	closed chan struct{}
}

func (w *wrapper) Listen() error {
	w.done = make(chan struct{})
	w.closed = make(chan struct{})
	if l, ok := w.Server.(srv.Listener); ok {
		return l.Listen()
	}
	return nil
}

func (w *wrapper) Serve(ctx context.Context) (err error) {
	selfexit := make(chan struct{})
	// Start the underlying server, monitor its exit by channel
	go func() {
		err = w.Server.Serve()
		close(selfexit)
	}()
	// React to either context cancel or self exit of server
	select {
	case <-ctx.Done():
		w.Server.Shutdown()
		<-selfexit
	case <-selfexit:
	}
	// If this server is also a srv.Waiter, Wait() for it an monitor how it reacts
	// If this server is Closed() without Wait() having exited, warn the user.
	if waiter, ok := w.Server.(srv.Waiter); ok {
		// monitor go-routine
		go func() {
			select {
			case <-w.done:
			case <-w.closed:
				ticker := time.NewTicker(time.Minute)
			POLL:
				for {
					select {
					case <-w.done:
						// waiter succeeded
						break POLL
					case <-ticker.C:
						Log(LvlWARN, fmt.Sprintf("srv.Server stuck in Wait()"))
					}
				}
				ticker.Stop()
			}
		}()
		// waiter go-routine
		go func() {
			waiter.Wait()
			close(w.done)
		}()
	} else {
		// This server is not a Waiter. It's done just by selfexit being closed.
		// which is already the case.
		close(w.closed)
		close(w.done)
	}
	return
}

func (w *wrapper) Shutdown(ctx context.Context) error {
	// Wait for the underlying done until context expires.
	// Then let the user decide what happens. - whether or not to call Close()
	select {
	case <-w.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *wrapper) Close() error {
	// We cannot enforce this, but we can close a channel to indicate
	// we want logged once/minute if the server is still stuck in Wait()
	select {
	case <-w.done:
	default:
		close(w.closed)
		return fmt.Errorf("Can't close  srv.Server")
	}
	return nil
}
