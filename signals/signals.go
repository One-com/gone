/*
Package signals provides a generic signal handler which will call simple functions based on a mapping from signal to function.

The signal handler will select over a set of signal channels being buffered by 1 signal. This means that a signal will only be lost of there's another similar signal pending.
*/
package signals

import (
	"os"
	"os/signal"
	"reflect"
)

// Action is a function called when an OS signal is received.
type Action func()

// Mappings map OS signals to functions
type Mappings map[os.Signal]Action

// Allocate a 1-buffered channel for each signal and do a select
// over all channels - has to use reflect for dynamic numbers of select cases.
func signalHandler(mappings Mappings) {

	cases := make([]reflect.SelectCase, len(mappings))
	actions := make([]Action, len(mappings))

	var idx = 0
	for sig, action := range mappings {
		sigch := make(chan os.Signal, 1)

		cases[idx].Dir = reflect.SelectRecv
		cases[idx].Chan = reflect.ValueOf(sigch)

		actions[idx] = action

		signal.Notify(sigch, sig)
		idx++
	}

	for {
		chosen, _, _ := reflect.Select(cases)
		f := actions[chosen]
		f()
	}
}

// RunSignalHandler spawns a go-routine which will call the provided Actions
// when receiving the corresponding signals.
func RunSignalHandler(m Mappings) {
	go signalHandler(m)
}
