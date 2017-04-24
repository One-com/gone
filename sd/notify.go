package sd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	unix "syscall"
	"time"
)

const (
	envNotifySocket = "NOTIFY_SOCKET"
	envWatchdogUsec = "WATCHDOG_USEC"
	envWatchdogPid  = "WATCHDOG_PID"
)

const (
	goneUnixSocketLockFdName = "GONEUXSCKLCK"
)

const (
	// StatusNone - Don't send a STATUS
	StatusNone = iota
	// StatusReady - Tell systemd status is READY
	StatusReady
	// StatusReloading - Tell systemd status is RELOADING
	StatusReloading
	// StatusStopping - Tell systemd status is STOPPING
	StatusStopping
	// StatusWatchdog - Tell the systemd WATCHDOG we are alive
	StatusWatchdog
)

const (
	// NotifyUnsetEnv flag provided to Notify() to unset the systemd notify/Watchdog env vars
	NotifyUnsetEnv = 1 << iota
	// NotifyWithFds flag to Notify() to instruct it to send active file descriptors along with
	// systemd notify message to FDSTORE
	NotifyWithFds
)

// ErrSdNotifyNoSocket is informs the caller that there's no NOTIFY_SOCKET available
var ErrSdNotifyNoSocket = errors.New("No systemd notify socket in environment")

var watchdogDuration time.Duration
var watchdogEnabled bool
var notifySocket string

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
		lines = append(lines, st)
	}
	lines = append(lines, msg)
	return Notify(0, lines...)
}

// Notify lets you control the message sent to the nofify socket more directly.
// flags control whether to unset the ENV and/or to include active file descriptors in
// the message for systemd to store in the FDSTORE
func Notify(flags int, lines ...string) (err error) {

	if flags&NotifyUnsetEnv != 0 {
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
		Name: fmt.Sprintf("\x00sdnotify%d", os.Getpid()),
		Net:  "unixgram",
	}

	var conn *net.UnixConn
	conn, err = net.ListenUnixgram("unixgram", abstract)
	if err != nil {
		return
	}
	defer conn.Close()

	state := strings.Join(lines, "\n")

	var oob []byte
	if flags&NotifyWithFds == 0 {
		_, _, err = conn.WriteMsgUnix([]byte(state), oob, socketAddr)
		return
	}

	// Do it with FDs
	// First send the message with any non-named FDs - then a message
	// for all the named ones - then a message for the locks
	
	var name2Fd  map[string][]int
	// make a map of all names needed to be sent -> slice of int
	for _, sdf := range fdState.activeFiles() {
		name := sdf.name
		slice := name2Fd[name]
		slice = append(slice, int(sdf.File.Fd()))
		name2Fd[name] = slice
		if sdf.lock != nil {
			slice := name2Fd[goneUnixSocketLockFdName]
			slice = append(slice, int(sdf.lock.Fd()))
			name2Fd[goneUnixSocketLockFdName] = slice
		}
	}

	// first add the empty named
	if expFiles, ok := name2Fd[""]; ok {
		delete(name2Fd, "")
		if state != "" {
			state += "\n"
		}
		state += "FDSTORE=1"
		oob = unix.UnixRights(expFiles...)
	}

	_, _, err = conn.WriteMsgUnix([]byte(state), oob, socketAddr)
	if err != nil {
		return
	}

	// Send the rest of the names, one message for each name
	for name, expFiles := range name2Fd {
		state = "FDSTORE=1\nFDNAME=" + name
		oob = unix.UnixRights(expFiles...)
		_, _, err = conn.WriteMsgUnix([]byte(state), oob, socketAddr)
		if err != nil {
			return
		}
	}
	return
}
