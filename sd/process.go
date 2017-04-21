package sd

import (
	"fmt"
	"os"

	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"sync"
)

const (
	envForkExecPid = "GONE_RESPAWN_PID"
	envForkExecSig = "GONE_RESPAWN_SIG"
)

// In order to keep the working directory the same as when we started we record
// it at startup.
var originalWD, _ = os.Getwd()

var startProcMu sync.Mutex

// StartProcess starts a new process passing it the open files. It
// starts a new process using the same environment and
// arguments as when it was originally started.
// This allows for a newly deployed binary to be started. It returns the pid of the newly started
// process when successful.
func StartProcess(env []string) (int, error) {
	startProcMu.Lock()
	defer startProcMu.Unlock()

	s := fdState

	files := s.activeFiles()

	// Use the original binary location. This works with symlinks such that if
	// the file it points to has been changed we will use the updated symlink.
	argv0, err := exec.LookPath(os.Args[0])
	if err != nil {
		return 0, err
	}

	// Pass on the environment, except fd fields
	for _, v := range os.Environ() {
		if !(strings.HasPrefix(v, envListenFds+"=") ||
			strings.HasPrefix(v, envListenFdNames+"=") ||
			strings.HasPrefix(v, envListenPid+"=") ||
			strings.HasPrefix(v, envGoneFdInfo+"=")) {
			env = append(env, v)
		}
	}
	env = append(env, fmt.Sprintf("%s=%d", envListenFds, len(files)))

	// Put info about FDs into ENV for new process
	var expFiles []*os.File
	envNames := envListenFdNames + "="
	envInfo  := envGoneFdInfo + "="
	for i, sdf := range files {
		if i != 0 {
			envNames += ":"
			envInfo += ":"
		}
		// if there's a lock  add the lock too, but mark it as a lock
		// for the next FD
		if sdf.lock != nil {
			expFiles = append(expFiles, sdf.lock)
			envInfo += "u:"
			envNames += ":"
		}
		expFiles = append(expFiles, sdf.File)
		envNames += sdf.name
	}
	env = append(env, envNames)
	env = append(env, envInfo)

	allFiles := append([]*os.File{os.Stdin, os.Stdout, os.Stderr}, expFiles...)
	process, err := os.StartProcess(argv0, os.Args, &os.ProcAttr{
		Dir:   originalWD,
		Env:   env,
		Files: allFiles,
	})
	if err != nil {
		return 0, err
	}

	return process.Pid, nil
}

// ReplaceProcess calls StartProcess, after setting up environment variables
// instructing the new process to signal the process calling ReplaceProcess
// for termination. To enable this signaling in the child process, call
// SignalParentTermination when ready.
func ReplaceProcess(sig syscall.Signal) (int, error) {
	var emptyenv [0]string
	return ReplaceProcessEnv(sig, emptyenv[:])
}

// ReplaceProcessEnv - like ReplaceProcess, but allows extra environment variables
// to be passed into the new instance
func ReplaceProcessEnv(sig syscall.Signal, env []string) (int, error) {
	pid := os.Getpid()

	env = env[0:len(env):len(env)] // if adding to the env, force copy to not modify original
	env = append(env, fmt.Sprintf("%s=%d", envForkExecPid, pid))
	if sig != syscall.Signal(0) {
		env = append(env, fmt.Sprintf("%s=%d", envForkExecSig, sig))
	}
	return StartProcess(env[:])
}


// SignalParentTermination signals any parent who have asked to be terminated via the ENV
func SignalParentTermination() error {
	var sig syscall.Signal = syscall.SIGTERM // default signal

	myparentstr := os.Getenv(envForkExecPid)
	if myparentstr == "" {
		return nil // nothing to do here
	}
	ppid := os.Getppid()
	mysigstr := os.Getenv(envForkExecSig)
	if mysigstr != "" {
		var err error
		var mysig int
		mysig, err = strconv.Atoi(mysigstr)
		if err != nil {
			//_log(LvlCRIT,"Failed parsing environment to signal parent", "err", err.Error())
			return err
		}
		sig = syscall.Signal(mysig)
	}

	myparent, err := strconv.Atoi(myparentstr)
	if err == nil && myparent == ppid && ppid != 1 {
		if err := syscall.Kill(ppid, sig); err != nil {
			//_log(LvlCRIT,"failed to close parent", "err", err.Error())
			return err
		}
	}
	// _log(LvlCRIT,"Failed parsing environment to signal parent", "err", err.Error())
	return err
}
