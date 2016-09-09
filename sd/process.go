package sd

import (
	"os"
	"os/exec"
	"syscall"
	"strings"
	"fmt"
	"strconv"
)

// In order to keep the working directory the same as when we started we record
// it at startup.
var originalWD, _  = os.Getwd()

// StartProcess starts a new process passing it the open files. It
// starts a new process using the same environment and
// arguments as when it was originally started.
// This allows for a newly deployed binary to be started. It returns the pid of the newly started
// process when successful.
func StartProcess(env []string) (int, error) {

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
		if ! (strings.HasPrefix(v, envListenFds+"=") ||
			strings.HasPrefix(v,envListenFdNames+"=") ||
			strings.HasPrefix(v,envListenPid+"=")) {
			env = append(env, v)
		}
	}
	env = append(env, fmt.Sprintf("%s=%d", envListenFds, len(files)))

	var expFiles []*os.File
	envNames := envListenFdNames + "="
	for i, sdf := range files {
		expFiles = append(expFiles, sdf.File)
		if i != 0 {
			envNames += ":"
		}
		envNames += sdf.name
	}

	env = append(env, envNames)
		
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
	pid := os.Getpid()
	var env [2]string
	var c int
	env[0] = fmt.Sprintf("NEWSTYLE_PID=%d",pid)
	if sig != syscall.Signal(0) {
		c = 1
		env[1] = fmt.Sprintf("NEWSTYLE_SIG=%d",sig)
	}
	return StartProcess(env[0:c])
}

// SignalParentTermination, signals any parent who have asked to be terminated via the ENV
func SignalParentTermination() error {
	var sig syscall.Signal = syscall.SIGTERM // default signal
	
	myparentstr := os.Getenv("NEWSTYLE_PID")
	if myparentstr == "" {
		return nil // nothing to do here
	}
	ppid := os.Getppid()
	mysigstr := os.Getenv("NEWSTYLE_SIG")
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

