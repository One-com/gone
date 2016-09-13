package sd

import (
	unix "syscall"
	"time"
	"fmt"
	"errors"
	"os"
	"net"
	"strings"
	"strconv"
)

const (
	envNotifySocket    = "NOTIFY_SOCKET"
	envWatchdogUsec    = "WATCHDOG_USEC"
	envWatchdogPid     = "WATCHDOG_PID"
)

const (
	// Don't send a STATUS
	StatusNone = iota
	// Tell systemd status is READY
	StatusReady
	// Tell systemd status is RELOADING
	StatusReloading
	// Tell systemd status is STOPPING
	StatusStopping
	// Tell the systemd WATCHDOG we are alive
	StatusWatchdog
)

const (
	// Unset the systemd notify/Watchdog env vars
	NotifyUnsetEnv = 1 << iota
	// Send active file descriptors along with systemd notify message to FDSTORE
	NotifyWithFds
)

// ErrSdNotifyNoSocket is informs the caller that there's no NOTIFY_SOCKET avaliable
var ErrSdNotifyNoSocket = errors.New("No systemd notify socket in environment")

var watchdogDuration time.Duration
var watchdogEnabled  bool
var notifySocket     string

func init() {
	if durStr := os.Getenv(envWatchdogUsec); durStr != "" {
		microsec, err := strconv.Atoi(durStr)
		if err == nil {
			watchdogDuration = time.Microsecond * time.Duration(microsec)
		}
	}
	if pidStr := os.Getenv(envWatchdogPid); pidStr != "" {
		if watchdogDuration != time.Duration(0) {
			if pidStr == "" {
				watchdogEnabled = true

			} else {
				pid, err := strconv.Atoi(pidStr)
				if err == nil && pid == os.Getpid() {
					watchdogEnabled = true
				}
			}
		}
	}
	if notifySocket = os.Getenv(envNotifySocket); notifySocket != "" {
		// Handle abstract sockets
		if notifySocket[0] == '@' {
			notifySocket = "\x00" + notifySocket[1:]
		}
	}
}

// WatchdogEnabled tell whether systemd asked us to enable watchdog notifications.
func WatchdogEnabled() (enabled bool, when time.Duration) {
	return watchdogEnabled, watchdogDuration
}

// NotifyStatus sends systemd the service status over the notify socket.
func NotifyStatus(status int, message string) error {
	msg := "STATUS=" + message
	var st string
	var lines []string
	switch status {
	case StatusNone:
	case StatusReady:
		st = "READY=1"
	case StatusReloading:
		st = "RELOADING=1"
	case StatusStopping:
		st = "STOPPING=1"
	case StatusWatchdog:
		st = "WATCHDOG=1"
	default:
		return errors.New("Unknown notify status")
	}
	if st != "" {
		lines = append(lines,st)
	}
	lines = append(lines,msg)
	return Notify(0, lines...)
}

// Notify lets you control the message sent to the nofify socket more directly.
// flags control whether to unset the ENV and/or to include active file descriptors in
// the message for systemd to store in the FDSTORE
func Notify(flags int, lines ...string) (err error) {

	if flags & NotifyUnsetEnv != 0 {
		defer os.Unsetenv(envNotifySocket)
	}

	if notifySocket == "" {
		return ErrSdNotifyNoSocket
	}

	socketAddr := &net.UnixAddr{
		Name: notifySocket,
		Net:  "unixgram",
	}


	abstract := &net.UnixAddr{
		Name: fmt.Sprintf("\x00sdnotify%d",os.Getpid()),
		Net:  "unixgram",
	}

	var conn *net.UnixConn
	conn, err = net.ListenUnixgram("unixgram",abstract)
	if err != nil {
		return
	}
	defer conn.Close()

	state := strings.Join(lines, "\n")

	var oob []byte
	if flags & NotifyWithFds != 0 {
		var expFiles []int
		var fdNames string
		for i, sdf := range fdState.activeFiles() {
			expFiles = append(expFiles, int(sdf.File.Fd()))
			if i != 0 {
				fdNames += ":"
			}
			fdNames += sdf.name
		}

		if state != "" {
			state += "\n"
		}
		state += "FDSTORE=1\nFDNAME=" + fdNames

		oob = unix.UnixRights(expFiles...)
	}

	_, _, err = conn.WriteMsgUnix([]byte(state), oob, socketAddr)
	return

}
