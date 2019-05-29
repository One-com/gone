package sd

import (
	"errors"
	"net"
	"os"
	"sync/atomic"
	unix "syscall"
)

// ErrNoSuchFdName signals an attempt to create a socket without specifying an existing fd name
// or an address. UNIX sockets require a file system path and can not (like IP socket) listen
// on all addresses, so it's an error to not provide either a name, or a path.
var ErrNoSuchFdName = errors.New("No file with the requested name and no requested address")

var unixSocketUnlinkPolicy uint32 = UnixSocketUnlinkPolicySocket

// UnixScoketUnlinkPolicy* constants are used to decide how to deal with listening on a UNIX socket where the filesystem file might already exist. One way is to just unlink it, but it's hard to know whether there's still a process using it. The flock(2) advisory locking scheme deals with this in a nice way.
const (
	// Never unlink socket files before listening
	UnixSocketUnlinkPolicyNone uint32 = iota
	// Always unlink socket files before listning
	UnixSocketUnlinkPolicyAlways
	// Always unlink, but stat(2) the file to ensure it's a UNIX socket before unlinking.
	UnixSocketUnlinkPolicySocket
	// Take a flock(2) lock on a socket.lock file before unlinking to ensure no other process is still using it.
	UnixSocketUnlinkPolicyFlock
)

// SetUnixSocketUnlinkPolicy decides how the sd library will deal with stale UNIX socket file when you create
// a new listening Unix socket which is not inherited from the environment the parent process (maybe systemd socket activation)
// made for your process.
//
// About UNIX domain sockets:
// UNIX sockets don't work exactly like TCP/UDP sockets. The kernel doesn't reclaim the "address" (ie. the socket file)
// when the last file descriptor is closed. The file still hangs around - and will prevent a new bind(2) to that
// adddress/path.
//
// Go has before version 1.7 solved this by doing unlink(2) on the file when you call Close() on a net.UnixListener.
// This doesn't play well with systemd socket activation though (or any graceful restart system transferring file descriptors).
//
// Systemd doesn't expect the file to disappear and removing it will prevent clients for connecting to the listener.
// See: https://github.com/golang/go/issues/13877
//
// Go 1.7 introduced some magic where listeners created with FileListener() would not do this.
//
// Go 1.8 allow the user to control this with SetUnlinkOnClose().
//
// The unix(7) manpage says: "Binding to a socket with a filename creates a socket in the filesystem that must be deleted by the caller when it is no longer needed".
//
// However...
//
// This may not be easy to guarantee... a process can crash before it removes the file. As evidence the same unix(7) man page
// exemplifies this with code where it calls unlink(2) just before bind(2) "In case the program exited inadvertently on the last run,
// remove the socket."
//
// Always unlinking the socket file might not be the thing you want either. There might be another instance of your daemon using it.
// Then unlinking would "steal" the address from the other process. This might be what you want - but you would have a small window
// without a socket file where clients would get rejected.
//
// So... the sd library will not try to unlink on close. In fact, it uses SetUnlinkOnClose(false) to never do this for any UNIX listener.
// Instead the sd library encourages "UnlinkBeforeBind". In other words: When it needs to create a new socket file it employs a UnixSocketUnlinkPolicy.
// This policy can be none (do not unlinK) or always unlink. Or it can be "test if the socket file is a socket file first".
// None of these will however "just work" in any scenario. Therefore there's a "use flock(2)" policy to use advisory file locking to ensure
// the socket file is only removed when there's no other process using it.
//
// The default is UnixSocketUnlinkPolicySocket. You need to change this ( like, in you init() ) to use the flock(2) mechanism.
func SetUnixSocketUnlinkPolicy(policy uint32) {
	atomic.StoreUint32(&unixSocketUnlinkPolicy, policy)
}

// InheritNamedListener returns a net.Listener and its systemd name passing
// the tests (and name criteria) provided.
// If there's no inherited FD which can be used the returned listener will be nil.
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func InheritNamedListener(wantName string, tests ...FileTest) (l net.Listener, gotName string, err error) {
	var file *os.File
	// look for an inherited listener
	file, gotName, err = FileWith(wantName, tests...)
	if err != nil {
		return
	}
	if file != nil {
		l, err = net.FileListener(file)
		if err != nil {
			return
		}
		err = Export(gotName, l)
		file.Close() // FileListener made a dup(). Export made a dup(). This copy is useless
	}
	return
}

// InheritNamedPacketConn returns a net.Listener and its systemd name passing
// the tests (and name criteria) provided.
// If there's no inherited FD which can be used the returned listener will be nil.
// The returned packetconn will be Export'ed by the sd library. Call Forget() to undo the export.
func InheritNamedPacketConn(wantName string, tests ...FileTest) (l net.PacketConn, gotName string, err error) {
	var file *os.File
	// look for an inherited listener
	file, gotName, err = FileWith(wantName, tests...)
	if err != nil {
		return
	}
	if file != nil {
		l, err = net.FilePacketConn(file)
		if err != nil {
			return
		}
		err = Export(gotName, l)
		file.Close() // FilePacketConn made a dup(). Export made a dup(). This copy is useless
	}
	return
}

// Listen returns a net.Listener like net.Listen, but will first check
// whether an inherited file descriptor is already listening before
// creating a new socket
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func Listen(nett, laddr string) (net.Listener, error) {
	switch nett {
	default:
		return nil, net.UnknownNetworkError(nett)
	case "tcp", "tcp4", "tcp6":
		addr, err := net.ResolveTCPAddr(nett, laddr)
		if err != nil {
			return nil, err
		}
		return ListenTCP(nett, addr)
	case "unix", "unixpacket":
		addr, err := net.ResolveUnixAddr(nett, laddr)
		if err != nil {
			return nil, err
		}
		return ListenUnix(nett, addr)
	}
}

// ListenPacket returns a net.PacketConn like net.ListenPacket, but will first
// check whether an inherited file descriptor is already available before
// creating a new socket
// The returned packetconn will be Export'ed by the sd library. Call Forget() to undo the export.
func ListenPacket(nett, laddr string) (net.PacketConn, error) {
	switch nett {
	default:
		return nil, net.UnknownNetworkError(nett)
	case "udp", "udp4", "udp6":
		addr, err := net.ResolveUDPAddr(nett, laddr)
		if err != nil {
			return nil, err
		}
		return ListenUDP(nett, addr)
	case "unixgram":
		addr, err := net.ResolveUnixAddr(nett, laddr)
		if err != nil {
			return nil, err
		}
		return ListenUnixgram(nett, addr)
	case "ip", "ip4", "ip6":
		addr, err := net.ResolveIPAddr(nett, laddr)
		if err != nil {
			return nil, err
		}
		return net.ListenIP(nett, addr)
	}
}

// ListenTCP returns a normal *net.TCPListener. It will create a new socket
// if there's no appropriate inherited file descriptor listening.
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func ListenTCP(nett string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	return NamedListenTCP("", nett, laddr)
}

// NamedListenTCP is like ListenTCP, but requires the file descriptor used to have
// the given systemd socket name
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func NamedListenTCP(name, nett string, laddr *net.TCPAddr) (l *net.TCPListener, err error) {

	var il net.Listener
	il, _, err = InheritNamedListener(name, IsTCPListener(laddr))
	if il != nil || err != nil {
		if err == nil {
			l = il.(*net.TCPListener)
		}
		return
	}

	// make a fresh listener
	l, err = net.ListenTCP(nett, laddr)
	if err != nil {
		return
	}
	err = Export(name, l)
	if err != nil {
		return
	}
	return
}

// ListenUnixgram returns a normal *net.UnixConn. It will create a new socket
// if there's no appropriate inherited file descriptor listening.
// The returned conn will be Export'ed by the sd library. Call Forget() to undo the export.
func ListenUnixgram(nett string, laddr *net.UnixAddr) (*net.UnixConn, error) {
	return NamedListenUnixgram("", nett, laddr)
}

// NamedListenUnixgram is like ListenUnixgram, but will require any used inherited
// file descriptor to have the given systemd socket name
// The returned conn will be Export'ed by the sd library. Call Forget() to undo the export.
func NamedListenUnixgram(name, nett string, laddr *net.UnixAddr) (l *net.UnixConn, err error) {

	var pathp *string
	if laddr != nil {
		pathp = &laddr.Name
	}

	var il net.PacketConn
	il, _, err = InheritNamedPacketConn(name, IsSocketUnix(unix.SOCK_DGRAM, 0, pathp))
	if il != nil || err != nil {
		if err == nil {
			l = il.(*net.UnixConn)
		}
		return
	}

	if laddr == nil {
		err = ErrNoSuchFdName
		return
	}

	lock, err := maybeUnlinkUnixSocketFile(laddr)
	if err != nil {
		// do nothing, let the bind fail
	}

	// make a fresh listener
	l, err = net.ListenUnixgram(nett, laddr)
	if err != nil {
		return
	}
	err = exportInternal(name, l, lock)
	if err != nil {
		l.Close()
		return
	}

	return
}

// ListenUnix is like net.ListenUnix and will return a normal *net.UnixListener.
// It will create a new socket
// if there's no appropriate inherited file descriptor listening.
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func ListenUnix(nett string, laddr *net.UnixAddr) (*net.UnixListener, error) {
	return NamedListenUnix("", nett, laddr)
}

// NamedListenUnix is like ListenUnix, but will require any used inherited
// file descriptor to have the given systemd socket name
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func NamedListenUnix(name, nett string, laddr *net.UnixAddr) (l *net.UnixListener, err error) {

	var il net.Listener
	il, _, err = InheritNamedListener(name, IsUNIXListener(laddr))
	if il != nil || err != nil {
		if err == nil {
			l = il.(*net.UnixListener)
		}
		return
	}

	if laddr == nil {
		err = ErrNoSuchFdName
		return
	}

	lock, err := maybeUnlinkUnixSocketFile(laddr)
	if err != nil {
		// do nothing, let the bind fail
	}

	// make a fresh listener
	l, err = net.ListenUnix(nett, laddr)
	if err != nil {
		return
	}
	l.SetUnlinkOnClose(false) /// we never do this. Leave it to unlink before bind
	err = exportInternal(name, l, lock)
	if err != nil {
		l.Close()
		return
	}
	return
}

// ListenUDP returns a normal *net.UDPConn. It will create a new socket
// if there's no appropriate inherited file descriptor listening. The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func ListenUDP(nett string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	return NamedListenUDP("", nett, laddr)
}

// NamedListenUDP is like ListenUDP, but will require any used inherited
// file descriptor to have the given systemd socket name.
// The returned listener will be Export'ed by the sd library. Call Forget() to undo the export.
func NamedListenUDP(name, net string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	return nil, nil
}

func maybeUnlinkUnixSocketFile(addr *net.UnixAddr) (lock *os.File, err error) {

	name := addr.Name

	policy := atomic.LoadUint32(&unixSocketUnlinkPolicy)

	if policy == UnixSocketUnlinkPolicyNone {
		return
	}

	if name[0] != '@' && name[0] != '\x00' {
		switch policy {
		case UnixSocketUnlinkPolicyAlways:
			err = unix.Unlink(name)
		case UnixSocketUnlinkPolicySocket:
			err = _statBeforeUnlink(name)
		case UnixSocketUnlinkPolicyFlock:

			lock, err = os.OpenFile(name+".lock", os.O_RDONLY|os.O_CREATE, 0700)
			if err != nil {
				return
			}
			// try acquire lock
			err = unix.Flock(int(lock.Fd()), unix.LOCK_EX|unix.LOCK_NB)
			if err == nil {
				err = _statBeforeUnlink(name)
			}
		}
	}
	return
}

func _statBeforeUnlink(name string) (err error) {

	var stat unix.Stat_t
	err = unix.Stat(name, &stat)
	if err == nil {
		if stat.Mode&unix.S_IFMT == unix.S_IFSOCK {
			err = unix.Unlink(name)
		}
	} else {
		if err == unix.ENOENT {
			err = nil // we have locked, but there was no socket file anyway
		}
	}
	return
}
