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

const (
	envGoneFdInfo      = "GONE_FDINFO"  // flags:flags:flags - only "u" flag defined
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
	name   string // fd name from systemd. This is *not* the same as presented to Open()
	lock *os.File // a potential flock(2) for UNIX socket file listeners
}

// close the file descriptor, if it's an owned UNIX domain socket
// unlink the socket file
func (f *sdfile) close() error {
	err := f.File.Close()
	return err
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
		defer os.Unsetenv(envGoneFdInfo)

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
			retErr = fmt.Errorf("Found invalid FD count in ENV: %s=%s", envListenFds, countStr)
			return
		}

		// Find any fd labels
		var names []string
		namesStr := os.Getenv(envListenFdNames)
		if namesStr != "" {
			names = strings.Split(namesStr, ":")
		}

		// Find additional gone/sd specific info about FD's (ie: Do the work as lock for Unix socket files?)
		var fdinfo []string
		fdinfoStr := os.Getenv(envGoneFdInfo)
		if fdinfoStr != "" {
			fdinfo = strings.Split(fdinfoStr, ":")
		}

		// Store the result as *os.File with name
		var nidx int
		var sum int
		var locksFromFdstore []int
		for fd := sdListenFdStart; fd < sdListenFdStart+count; fd++ {
			var lock *os.File
			var newfilename string
			// first check if it's UNIX socket file lock
			var listeningUnixSocket bool
			if fdinfo != nil && fdinfo[nidx] == "u" {
				if path, ok := listeningUnixSocketPath(fd+1); ok {
					lock = os.NewFile(uintptr(fd), path + ".lock")
					newfilename = path
					unix.CloseOnExec(fd)
				} else {
					retErr = unix.Close(fd)
				}
				fd++ // advance to the next fd which this lock is protecting
				nidx++
			}
			// then check if it's a lock from FDSTORE
			var nm string // name to make this FD available under
			if names != nil {
				if names[nidx] == goneUnixSocketLockFdName {
					locksFromFdstore = append(locksFromFdstore, fd)
					continue
				} else {
					// normal name - save it
					nm = names[nidx]
					s.names = append(s.names, nm)
				}
			}

			unix.CloseOnExec(fd) // unless this FD is explicitly re-exported, we close it on exec
			if listeningUnixSocket {
				// don't touch newfilename
			} else {
				newfilename = "fd:"+nm // Not sure if anyone relies on this being addrinfo?
			}
			file := os.NewFile(uintptr(fd), newfilename)
			sdf := &sdfile{name: nm, File: file, lock: lock}

			s.available = append(s.available, sdf)
			nidx++
			sum++
		}
		// hook fdstore locks up on the socket fds
		if locksFromFdstore != nil {
			for _, sdf := range s.available {
				if path, ok := listeningUnixSocketPath(int(sdf.Fd())); ok {
					if sdf.lock == nil { // find any lock
						var st unix.Stat_t
						lockpath := path + ".lock"
						err = unix.Stat(lockpath, &st)
						if err != nil {
							if err == unix.ENOENT {
								continue // there's no lock for this socket
							}
							retErr = err
							return
						}
						for _, lockfd := range locksFromFdstore {
							var stl unix.Stat_t
							err = unix.Fstat(lockfd, &stl)
							if err != nil {
								retErr = err
								return
							}
							if st.Dev == stl.Dev && st.Ino == stl.Ino {
								// this is the lock for this socket. put it in
								sdf.lock = os.NewFile(uintptr(lockfd), lockpath)
							}
						}
					} // else we have unreachable code.
				}
			}
		}
		// Make inherited FDs available (TODO: flocks counted?)
		s.count = sum
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
				if file.lock != nil {
					file.lock.Close()
				}
				s.active[i] = nil
				delete(s.active, i)
			}
		}
	default:
		// Look up the object and close the file descriptor we have for it.
		if file, ok := s.active[f]; ok {
			file.File.Close()
			if file.lock != nil {
				file.lock.Close()
			}
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
	err = exportInternal(sdname, f, nil)
	return
}

func exportInternal(sdname string, f interface{}, lock *os.File) (err error) {

	s := fdState
	var file *os.File

	// Make sure we keep a dup(2) of the FD
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

	// Store the resulting *os.File in the active map
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if _, already := s.active[f]; !already {
		s.active[f] = &sdfile{File: file, name: sdname, lock: lock}
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
	fdState.mutex.Lock()
	defer fdState.mutex.Unlock()
	return fdState.count, fdState.names, fdState.err
}
