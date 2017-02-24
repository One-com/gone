package sd

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	unix "syscall"
)

const (
	// Used to indicate a graceful restart in the new process.
	envListenFds       = "LISTEN_FDS"
	envListenPid       = "LISTEN_PID"
	envListenFdNames   = "LISTEN_FDNAMES"
	envIgnoreListenPid = "LISTEN_PID_IGNORE" // for testing
	sdListenFdStart    = 3
)

var fdState *state

// The library can manage either *os.File objects or
// objects which can supply an *os.File via a dup() by calling File()
type filer interface {
	File() (*os.File, error)
}

// To keep the systemd label of the file descriptor with the file
type sdfile struct {
	*os.File
	name string // fd name from systemd. This is *not* the same as presented to Open()
}

func (f *sdfile) close() error {
	return f.File.Close()
}

// state of file descriptors going in/out to systemd
type state struct {
	mutex       sync.Mutex
	inheritOnce sync.Once

	err   error
	count int
	names []string

	available []*sdfile

	// When an available *sdfile is use it's recorded here.
	// These (if not closed) are also the *os.File exported
	// a map removed duplicates
	active map[interface{}]*sdfile
}

func newState() (s *state) {
	s = &state{}
	s.active = make(map[interface{}]*sdfile)
	return
}

func init() {
	fdState = newState()
	fdState.inherit()
}

// return a copy of the active file slice
func (s *state) activeFiles() []*sdfile {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s._activeFiles()
}

// to be called already under mutex lock
func (s *state) _activeFiles() []*sdfile {
	ls := make([]*sdfile, len(s.active))
	var i int
	for _, sd := range s.active {
		if sd != nil {
			ls[i] = sd
			i++
		}
	}
	return ls
}

func _activefds() (ret []uintptr) {
	list := fdState.activeFiles()
	for _, sd := range list {
		ret = append(ret, sd.File.Fd())
	}
	return
}

func _availablefds() (ret []uintptr) {

	fdState.mutex.Lock()
	defer fdState.mutex.Unlock()

	for _, sd := range fdState.available {
		if sd != nil {
			ret = append(ret, sd.File.Fd())
		}
	}
	return
}


// Cleanup closes all inherited file descriptors which have not been Exported
func Cleanup() {
	fdState.cleanup()
}

// Cleanup closes all inactive files
func (s *state) cleanup() {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	s._cleanupLocked()
}


func (s* state) _cleanupLocked() {
	for _, f := range s.available {
		if f != nil {
			f.close()
		}
	}
	s.available = nil
}

// Reset closes all inherited an non Exported file descriptors and makes the current
// Exported set of file descriptors avaible again as if they were inherited.
func Reset() {
	fdState.reset()
}

// Reset closes all non-active files and all returned listeners/packetconns/files
// and makes the current active files available again
func (s *state) reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// close all FD's not actually used
	s._cleanupLocked()

	// make the current active FDs available again
	s.available = s._activeFiles()

	// update starting point data
	var nidx int
	s.names = nil
	for _, sd := range s.available {
		if sd != nil {
			s.names = append(s.names, sd.name)
			nidx++
		}
	}

	s.count = nidx
	s.err = nil

	// reset the active FDs' to be empty
	s.active = make(map[interface{}]*sdfile)
}

func (s *state) inherit() error {
	var retErr error

	s.inheritOnce.Do(func() {

		defer os.Unsetenv(envListenPid)
		defer os.Unsetenv(envListenFds)
		defer os.Unsetenv(envListenFdNames)

		countStr := os.Getenv(envListenFds)
		if countStr == "" {
			// No inherited fds
			return
		}

		// Check the file descriptors are for us
		// NB: We cannot set the PID when respawning due to Go limitations.
		if pidStr := os.Getenv(envListenPid); pidStr != "" {
			pid, err := strconv.Atoi(pidStr)
			if err != nil {
				retErr = err
				return
			}
			if pid != os.Getpid() {
				// not for us
				ignore := os.Getenv(envIgnoreListenPid)
				if ignore == "" {
					fmt.Println("Not for us", pid)
					return
				}
			}
		}

		count, err := strconv.Atoi(countStr)
		if err != nil {
			retErr = fmt.Errorf("found invalid count value: %s=%s", envListenFds, countStr)
			return
		}

		// Find any fd labels
		var names []string
		namesStr := os.Getenv(envListenFdNames)
		if namesStr != "" {
			names = strings.Split(namesStr, ":")
		}

		// Store the result as *os.File with name
		var nidx int
		for fd := sdListenFdStart; fd < sdListenFdStart+count; fd++ {
			var nm string
			if names != nil {
				nm = names[nidx]
				s.names = append(s.names, nm)
			}
			nidx++
			unix.CloseOnExec(fd)

			file := os.NewFile(uintptr(fd), "fd:"+nm)
			sdf := &sdfile{name: nm, File: file}
			s.available = append(s.available, sdf)
		}

		s.count = nidx
	})
	s.err = retErr
	return retErr
}

// Forget makes the sd library forget about its file descriptor (made by Export)
// associated with either an exported object or a string naming a file descriptor.
// If more file descriptors are named the same, they are all closed.
// Forget should be passed, either a string naming the file descriptor, OR the object
// originally exported.
func Forget(f interface{}) (err error) {
	s := fdState

	s.mutex.Lock()
	defer s.mutex.Unlock()

	switch str := f.(type) {
	case string:
		// Close all files with that systemd name
		for i, file := range s.active {
			if file != nil && file.name == str {
				file.File.Close()
				s.active[i] = nil
				delete(s.active, i)
			}
		}
	default:
		// Look up the object and close the file descriptor we have for it.
		if file, ok := s.active[f]; ok {
			file.File.Close()
			s.active[f] = nil
			delete(s.active, f)
		} else {
			err = errors.New("File descriptor not exported")
		}
	}
	return
}

// Export records a dup() of the file descriptor and makes it managed by the sd package, marked
// as in active use. Closing the object provided will not close the managed file descriptor, so
// any socket connection will still be open an be able to be transferred to other processes/objects
// in open state.
// If you want to stop managing the file descriptor and close it, call Forget() on the name, or provided
// the same object as was exported.
func Export(sdname string, f interface{}) (err error) {
	s := fdState
	var file *os.File
	switch tf := f.(type) {
	case *os.File:
		var newfd int
		newfd, err = dupCloseOnExec(int(tf.Fd()))
		if err != nil {
			return
		}
		file = os.NewFile(uintptr(newfd), tf.Name())
	case filer:
		// File() already does the dup
		file, err = tf.File()
		if err != nil {
			return
		}
		// The Go net package sets the socket blocking.
		err = unix.SetNonblock(int(file.Fd()), true)
		if err != nil {
			file.Close()
			return
		}
	default:
		err = errors.New("Unknown file type. Not exported")
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, already := s.active[f]; !already {
		s.active[f] = &sdfile{File: file, name: sdname}
	} else {
		// This probably shouldn't happen, since it would mean we had gotten an
		// already used fd from dup()
		file.Close()
		err = errors.New("File descriptor already exported")
	}
	return
}

func dupCloseOnExec(fd int) (newfd int, err error) {
	return fcntl(fd, unix.F_DUPFD_CLOEXEC, 0)
}

func fcntl(fd int, cmd int, arg int) (val int, err error) {
	r0, _, e1 := unix.Syscall(unix.SYS_FCNTL, uintptr(fd), uintptr(cmd), uintptr(arg))
	val = int(r0)
	if e1 != 0 {
		err = errors.New("DUP error")
	}
	return
}

// FileWith returns any file descriptor (as an *os.File) which matches the given systemd name
// *and* any FileTests provided from the available pool of (inherited) file descriptors.
// The file descriptor is marked as no longer available and forgotten by the library.
// If the name requested is "", any file descriptor matching the tests is returned.
// The actual name is also returned FYI (in case the requested name was "")
// The name provided here is *NOT* the same name as the calling .Name() on the returned file.
// This name is the systemd name as controlled by the systemd socket unit FileDescriptorName=
// Calling .Name() on an socket *os.File will usually return information about bound addresses.
// Notice: Once the file is returned, it's no longer the responsibility of the sd package, so any
// test to determine whether the file is actually the one you need should be defined and passed as
// a FileTest. You cannot get a file based on "half-a-test", do some more testing later and
// then regret and put it back. Write a FileTest function instead.
// If you want the sd library to track the returned file as in active use, call Export() on it.
func FileWith(sdname string, tests ...FileTest) (rfile *os.File, rname string, err error) {
	s := fdState
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for i, candidate := range s.available {
		if candidate == nil { // we nil used inherited files
			continue
		}
		if sdname != "" && candidate.name != sdname {
			continue
		}
		var ok bool
		if ok, err = candidate.isMatching(tests...); ok && err == nil {
			rfile = candidate.File
			rname = candidate.name
			s.available[i] = nil
			return
		}
		if err != nil {
			return
		}
	}
	return
}

// ListenFdsWithNames return the number of inherited filedescriptors and their names, along with any error
// occurring while inheriting them.
// Calling Reset() will reset these values too.
func ListenFdsWithNames() (count int, names []string, err error) {
	return fdState.count, fdState.names, fdState.err
}
