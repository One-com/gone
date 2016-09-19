package sd

import (
	"errors"
	"net"
	"os"
	unix "syscall"
)

var listenerBacklog = maxListenerBacklog()

// InheritNamedListener returns a net.Listener and its systemd name passing
// the tests (and name criteria) provided.
// If there's no inherited FD which can be used the returned listener will be nil.
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
		err = Export(gotName, file)
		file.Close() // FileListener made a dup(). Export made a dup(). This copy is useless
	}
	return
}

// InheritNamedPacketConn returns a net.Listener and its systemd name passing
// the tests (and name criteria) provided.
// If there's no inherited FD which can be used the returned listener will be nil.
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
		err = Export(gotName, file)
		file.Close() // FilePacketConn made a dup(). Export made a dup(). This copy is useless
	}
	return
}

// Listen returns a net.Listener like net.Listen, but will first check
// whether an inherited file descriptor is already listening before
// creating a new socket
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
func ListenTCP(nett string, laddr *net.TCPAddr) (*net.TCPListener, error) {
	return NamedListenTCP("", nett, laddr)
}

// NamedListenTCP is like ListenTCP, but requires the file descriptor used to have
// the given systemd socket name
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
func ListenUnixgram(nett string, laddr *net.UnixAddr) (*net.UnixConn, error) {
	return NamedListenUnixgram("", nett, laddr)
}

// NamedListenUnixgram is like ListenUnixgram, but will require any used inherited
// file descriptor to have the given systemd socket name
func NamedListenUnixgram(name, nett string, laddr *net.UnixAddr) (*net.UnixConn, error) {
	return nil, nil
}

// ListenUnix is like net.ListenUnix and will return a normal *net.UnixListener.
// It will create a new socket
// if there's no appropriate inherited file descriptor listening.
func ListenUnix(nett string, laddr *net.UnixAddr) (*net.UnixListener, error) {
	return NamedListenUnix("", nett, laddr)
}

// NamedListenUnix is like ListenUnix, but will require any used inherited
// file descriptor to have the given systemd socket name
func NamedListenUnix(name, nett string, laddr *net.UnixAddr) (l *net.UnixListener, err error) {

	var il net.Listener
	il, _, err = InheritNamedListener(name, IsUNIXListener(laddr))
	if il != nil || err != nil {
		if err == nil {
			l = il.(*net.UnixListener)
		}
		return
	}

	// make a fresh listener
	l, err = noUnlinkUnixListener(nett, laddr)
	if err != nil {
		return
	}
	err = Export(name, l)
	if err != nil {
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

/*===================================================================================*/
// Code to handle the sad fact the the standard library does unlink on Unix listeners
// when they are closed. We need to obtain a UNIX listener via net.FileListener
// which make netFDs not doing that.
// So a lot of duplicated code.

// We have take this over from the stdlib, since the net package thinks it's a good idea to
// call unlink() in Close() for UnixListener !?!?!?
func noUnlinkUnixListener(nett string, laddr *net.UnixAddr) (l *net.UnixListener, err error) {
	var sotype int

	sotype, err = net2sotypeUnix(nett)
	if err != nil {
		return
	}

	var fd int
	fd, err = socket(nett, unix.AF_UNIX, sotype, 0, false, laddr)
	if err != nil {
		return
	}

	var lis net.Listener
	file := os.NewFile(uintptr(fd), "")
	lis, err = net.FileListener(file)
	file.Close()
	if err != nil {
		return
	}

	var ok bool
	if l, ok = lis.(*net.UnixListener); !ok {
		err = errors.New("Could not create no-unlink UNIX listener")
	}

	return
}

func socket(net string, family, sotype, proto int, ipv6only bool, laddr *net.UnixAddr) (fd int, err error) {
	if laddr == nil {
		err = errors.New("Can't make UNIX listener with nil address")
		return
	}

	var s int
	s, err = unix.Socket(family, sotype|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC, proto)
	if err != nil {
		return
	}
	if err = setDefaultSockopts(s, family, sotype, ipv6only); err != nil {
		unix.Close(s)
		return
	}

	// This function makes a network file descriptor for the
	// following applications:
	//
	// - An endpoint holder that opens a passive stream
	//   connection, known as a stream listener
	//
	// - An endpoint holder that opens a destination-unspecific
	//   datagram connection, known as a datagram listener
	//
	// - An endpoint holder that opens an active stream or a
	//   destination-specific datagram connection, known as a
	//   dialer
	//
	// - An endpoint holder that opens the other connection, such
	//   as talking to the protocol stack inside the kernel
	//
	// For stream and datagram listeners, they will only require
	// named sockets, so we can assume that it's just a request
	// from stream or datagram listeners when laddr is not nil but
	// raddr is nil. Otherwise we assume it's just for dialers or
	// the other connection holders.

	switch sotype {
	case unix.SOCK_STREAM, unix.SOCK_SEQPACKET:
		if err = listenStream(s, laddr, listenerBacklog); err != nil {
			unix.Close(s)
			return
		}
	case unix.SOCK_DGRAM:
		if err = listenDatagram(s, laddr); err != nil {
			unix.Close(s)
			return
		}
	}
	fd = s
	return
}

func listenStream(fd int, laddr *net.UnixAddr, backlog int) error {
	if err := setDefaultListenerSockopts(fd); err != nil {
		return err
	}
	lsa := &unix.SockaddrUnix{Name: laddr.Name}

	if err := unix.Bind(fd, lsa); err != nil {
		// The bind() error might be due to us not having unlinked the file
		// when we last stopped using it.
		// But we can't know when the last us is, since a server should bother
		// about whether it created the socket it self or got it from the ENV/systemd
		// So, ... assume here that the reason we are here at all is that there was
		// no fitting UNIX listener socket in the ENV, so if the file is still here
		// it's a stale file which should be unlinked before binding again.
		// But do some basics tests that you don't just unlink anything.
		var stat unix.Stat_t
		var e2 error
		e2 = unix.Stat(lsa.Name, &stat)
		if e2 == nil {
			if stat.Mode&unix.S_IFMT == unix.S_IFSOCK {
				// Try unlink it
				e2 = unix.Unlink(lsa.Name)
				if e2 == nil {
					// Try again
					err = unix.Bind(fd, lsa)
				}
			}
		}
		if e2 != nil {
			err = os.NewSyscallError("bind", e2)
		}
		if err != nil {
			return os.NewSyscallError("bind", err)
		}
	}
	if err := unix.Listen(fd, backlog); err != nil {
		return os.NewSyscallError("listen", err)
	}
	return nil
}

func listenDatagram(fd int, laddr *net.UnixAddr) error {
	lsa := &unix.SockaddrUnix{Name: laddr.Name}

	if err := unix.Bind(fd, lsa); err != nil {
		return os.NewSyscallError("bind", err)
	}
	return nil
}

func setDefaultListenerSockopts(s int) error {
	// Allow reuse of recently-used addresses.
	return os.NewSyscallError("setsockopt", unix.SetsockoptInt(s, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1))
}

func setDefaultSockopts(s, family, sotype int, ipv6only bool) error {
	if family == unix.AF_INET6 && sotype != unix.SOCK_RAW {
		// Allow both IP versions even if the OS default
		// is otherwise.  Note that some operating systems
		// never admit this option.
		unix.SetsockoptInt(s, unix.IPPROTO_IPV6, unix.IPV6_V6ONLY, boolint(ipv6only))
	}
	// Allow broadcast.
	return os.NewSyscallError("setsockopt", unix.SetsockoptInt(s, unix.SOL_SOCKET, unix.SO_BROADCAST, 1))
}

// Boolean to int.
func boolint(b bool) int {
	if b {
		return 1
	}
	return 0
}

func maxListenerBacklog() int {
	return unix.SOMAXCONN
}
