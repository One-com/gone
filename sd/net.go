package sd

import (
	"errors"
	"net"
	"os"
	unix "syscall"
	"fmt"
)

var ErrNoSuchFdName = errors.New("No file with the requested name and no requested address")

var unixSocketUnlinkPolicy int

const (
	unixSocketUnlinkPolicyNone int = iota
	unixSocketUnlinkPolicyAlways
	unixSocketUnlinkPolicySocket
	unixSocketUnlinkPolicyFlock
)

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
func NamedListenUnixgram(name, nett string, laddr *net.UnixAddr) (*net.UnixConn, error) {
	return nil, nil
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
	fmt.Println("fresh", lock)
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
// if there's no appropriate inherited file descriptor listening.
func ListenUDP(net string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	return NamedListenUDP("", net, laddr)
}

// NamedListenUDP is like ListenUDP, but will require any used inherited
// file descriptor to have the given systemd socket name
func NamedListenUDP(name, net string, laddr *net.UDPAddr) (*net.UDPConn, error) {
	return nil, nil
}

func maybeUnlinkUnixSocketFile(addr *net.UnixAddr) (lock *os.File, err error) {
	var stat unix.Stat_t
	name := addr.Name
	if name[0] != '@' && name[0] != '\x00' {
		fmt.Println("name"+ name)
		lock, err = os.OpenFile(name+".lock", os.O_RDONLY|os.O_CREATE, 0700)
		if err != nil {
			return
		}
		// try aquire lock
		err = unix.Flock(int(lock.Fd()), unix.LOCK_EX | unix.LOCK_NB)
		if err == nil {
			fmt.Println("locked")
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
		} else {
			fmt.Println("error"+err.Error())
		}
	}
	return
}
