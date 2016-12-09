# gone signals

Run a signal handler and call functions based on a signal->function map.

Each signal will be notified on it's own 1-buffered channel, so no signal will
be lost unless there's an equal signal pending.

## Example

```go
import (
	"github.com/One-com/gone/signals"
    "os"
	"syscall"
)

// ... define your functions

func init() {

    handledSignals := signals.Mappings{
        syscall.SIGINT  : onSignalExit,
		syscall.SIGTERM : onSignalExitGraceful,
		syscall.SIGHUP  : onSignalReload,
		syscall.SIGUSR2 : onSignalRespawn,
		syscall.SIGTTIN : onSignalIncLogLevel,
		syscall.SIGTTOU : onSignalDecLogLevel,
	}

	signals.RunSignalHandler(handledSignals)
}
```
