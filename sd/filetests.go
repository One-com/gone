package sd

import (
	"errors"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"
	unix "syscall"
)

/*
* Providing tests like
* https://www.freedesktop.org/software/systemd/man/sd_is_socket.html
*
 */

// FileTest is a function returning whether an *os.File fulfills certain criteria.
// You can write these your self - and should, if the provided ones don't cover your requirements
// fully.
type FileTest func(*os.File) (bool, error)

func (f *sdfile) isMatching(tests ...FileTest) (ok bool, err error) {
	for _, t := range tests {
		if ok, err = t(f.File); err != nil {
			return
		}
		if !ok {
			return
		}
	}
	// all tests succeeded
	ok = true
	return
}

//--------------------------------------------------------------------

func isSocketInternal(fd uintptr, sotype int, want_listening int) (ok bool, err error) {
	var stat unix.Stat_t
	err = unix.Fstat(int(fd), &stat)
	if err != nil {
		return
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFSOCK {
		return
	}

	var istype int
	if sotype != 0 {
		istype, err = unix.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_TYPE)
		if err != nil {
			return
		}
		if sotype != istype {
			return
		}

	}

	if want_listening >= 0 {
		var val int
		// Test listening
		val, err = unix.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_ACCEPTCONN)
		if err != nil {
			return
		}
		is := val != 0
		want := want_listening > 0
		if is != want {
			return
		}
		ok = true
	}
	return
}

//-------------------------------------------------------------------------------

// IsFifo tests whether the *os.File is a FIFO at the given path
func IsFifo(path string) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		var stat unix.Stat_t
		err = unix.Fstat(int(fd), &stat)
		if err != nil {
			return
		}

		if stat.Mode&unix.S_IFMT != unix.S_IFIFO {
			return
		}

		if path != "" {
			var pstat unix.Stat_t
			err = unix.Stat(path, &pstat)
			if err != nil {
				if err == unix.ENOENT || err == unix.ENOTDIR {
					err = nil
					return
				}
				return
			}
			ok = stat.Dev == pstat.Dev && stat.Ino == pstat.Ino
			return
		}

		ok = true
		return
	}
}

func listeningUnixSocketPath(fd int) (path string, ok bool) {
	ok, err := isSocketInternal(uintptr(fd), 0, 1) // we want a listening UNIX socket of any sotype.
	if !ok || err != nil {
		return
	}
	var lsa unix.Sockaddr
	lsa, err = unix.Getsockname(fd)
	if err != nil {
		if err == unix.ENOTSOCK {
			err = nil
		}
		return
	}
	if ua, ok := lsa.(*unix.SockaddrUnix); ok {
		return ua.Name, true
	}
	return
}

// IsSocket is similar to libsystemd sd-daemon sd_is_socket()
// Test whether the *os.File is a socket of family, socket type and whether it's listening.
// Values for "dont' care" is respectively 0, 0, and -1
func IsSocket(family, sotype int, listening int) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()
		ok, err = isSocketInternal(fd, sotype, listening)
		if !ok || err != nil {
			return
		}
		if family > 0 {
			ok = false
			var lsa unix.Sockaddr
			var lsf int
			lsa, err = unix.Getsockname(int(fd))
			if err != nil {
				return
			}
			switch lsa.(type) {
			case *unix.SockaddrInet4:
				lsf = unix.AF_INET
			case *unix.SockaddrInet6:
				lsf = unix.AF_INET6
			case *unix.SockaddrUnix:
				lsf = unix.AF_UNIX
			case *unix.SockaddrNetlink:
				lsf = unix.AF_NETLINK
			default:
				err = errors.New("Socket has unsupported address family")
				return
			}
			if family == lsf {
				ok = true
			}
		}
		return
	}
}

// IsSocketInet is similar to sd_is_socket_inet
// Test if the *os.File is a socket of AF_INET/AF_INET6 of the given socket time, listening (0,1,-1) and port
func IsSocketInet(family int, sotype int, listening int, port uint16) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		ok, err = isSocketInternal(fd, sotype, listening)
		if !ok || err != nil {
			return
		}

		// check the family
		if family != unix.AF_INET && family != unix.AF_INET6 {
			return
		}

		lsa, _ := unix.Getsockname(int(fd))

		switch a := lsa.(type) {
		case *unix.SockaddrInet4:
			if family != unix.AF_INET {
				return
			}
			if port > 0 {
				if int(port) != a.Port {
					return
				}
			}
		case *unix.SockaddrInet6:
			if family != unix.AF_INET6 {
				return
			}
			if port > 0 {
				if int(port) != a.Port {
					return
				}
			}

		default:
			return
		}
		ok = true
		return
	}
}

// IsSocketUnix is like libsystemd sd-daemon sd_is_socket_unix()
// path is a pointer to allow for "don't care" option by passing nil pointer.
// The empty string is the unnamed socket.
func IsSocketUnix(sotype int, listening int, path *string) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		ok, err = isSocketInternal(fd, sotype, listening)
		if !ok || err != nil {
			return
		}

		// check that it's an AF_UNIX
		var lpath string
		lsa, _ := unix.Getsockname(int(fd))

		switch a := lsa.(type) {
		case *unix.SockaddrUnix:
			lpath = a.Name
		default:
			return
		}

		// Check the path if requested
		if path != nil && *path != lpath {
			return
		}

		ok = true
		return
	}
}

// IsUNIXListener tests if the *os.File is a unix(7) socket listening on the given address.
// Setting addr == nil, means "any" AF_UNIX address.
// Including linux abstract sockets.
func IsUNIXListener(addr *net.UnixAddr) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()
		var sotype int
		sotype, err = net2sotypeUnix(addr.Network())
		if err != nil {
			return
		}

		ok, err = isSocketInternal(fd, sotype, 1)
		if !ok || err != nil {
			return
		}

		if addr != nil {
			ok = false
			// Check the address
			var saddr net.Addr
			lsa, _ := unix.Getsockname(int(fd))

			switch lsa.(type) {
			case *unix.SockaddrUnix:
				saddr = addrFunc(unix.AF_UNIX, sotype)(lsa)
			default:
				return
			}

			ok = isSameUnixAddr(saddr, addr)
		}
		return
	}
}

// IsTCPListener tests whether the *os.File is a listening TCP socket.
// If addr != nil it's tested whether it is bound to that address.
func IsTCPListener(addr *net.TCPAddr) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		sotype := unix.SOCK_STREAM

		ok, err = isSocketInternal(fd, sotype, 1)
		if !ok || err != nil || addr == nil {
			return
		}

		ok = false
		// Check the address if it was not nil
		var saddr net.Addr
		lsa, _ := unix.Getsockname(int(fd))

		switch lsa.(type) {
		case *unix.SockaddrInet4:
			saddr = addrFunc(unix.AF_INET, sotype)(lsa)
		case *unix.SockaddrInet6:
			saddr = addrFunc(unix.AF_INET6, sotype)(lsa)
		default:
			return
		}

		ok = isSameIPAddr(saddr, addr)
		return
	}
}

// IsUDPListener is like IsTCPListener, but for UDP
func IsUDPListener(addr *net.UDPAddr) FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		sotype := unix.SOCK_DGRAM

		ok, err = isSocketInternal(fd, sotype, 1)
		if !ok || err != nil || addr == nil {
			return
		}

		ok = false
		// Check the address if it was not nil
		var saddr net.Addr
		lsa, _ := unix.Getsockname(int(fd))

		switch lsa.(type) {
		case *unix.SockaddrInet4:
			saddr = addrFunc(unix.AF_INET, sotype)(lsa)
		case *unix.SockaddrInet6:
			saddr = addrFunc(unix.AF_INET6, sotype)(lsa)
		default:
			return
		}

		ok = isSameIPAddr(saddr, addr)
		return
	}
}

// IsListening FileTest to determine whether a file descriptor is a listening socket or not.
func IsListening(want bool) FileTest {
	return func(file *os.File) (bool, error) {
		fd := file.Fd()
		var w int
		if want {
			w = 1
		}
		return isSocketInternal(fd, 0, w)
	}
}

// IsSoReusePort - Test if SO_REUSEPORT is set on the socket
func IsSoReusePort() FileTest {
	return func(file *os.File) (ok bool, err error) {
		fd := file.Fd()

		ok, err = isSocketInternal(fd, 0, -1)
		if !ok && err != nil {
			return
		}

		val, err := unix.GetsockoptInt(int(fd), syscall.SOL_SOCKET, reusePort)
		if err != nil {
			return
		}
		if val == 1 {
			ok = true
		}
		return
	}
}

//--------------------------------------------------------------------------------

func sockaddrToTCP(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		return &net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port}
	case *unix.SockaddrInet6:
		return &net.TCPAddr{IP: sa.Addr[0:], Port: sa.Port, Zone: zoneToString(int(sa.ZoneId))}
	}
	return nil
}

func sockaddrToUDP(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		return &net.UDPAddr{IP: sa.Addr[0:], Port: sa.Port}
	case *unix.SockaddrInet6:
		return &net.UDPAddr{IP: sa.Addr[0:], Port: sa.Port, Zone: zoneToString(int(sa.ZoneId))}
	}
	return nil
}

func sockaddrToUnix(sa unix.Sockaddr) net.Addr {
	if s, ok := sa.(*unix.SockaddrUnix); ok {
		return &net.UnixAddr{Name: s.Name, Net: "unix"}
	}
	return nil
}

func sockaddrToUnixgram(sa unix.Sockaddr) net.Addr {
	if s, ok := sa.(*unix.SockaddrUnix); ok {
		return &net.UnixAddr{Name: s.Name, Net: "unixgram"}
	}
	return nil
}

func sockaddrToUnixpacket(sa unix.Sockaddr) net.Addr {
	if s, ok := sa.(*unix.SockaddrUnix); ok {
		return &net.UnixAddr{Name: s.Name, Net: "unixpacket"}
	}
	return nil
}

func sockaddrToIP(sa unix.Sockaddr) net.Addr {
	switch sa := sa.(type) {
	case *unix.SockaddrInet4:
		return &net.IPAddr{IP: sa.Addr[0:]}
	case *unix.SockaddrInet6:
		return &net.IPAddr{IP: sa.Addr[0:], Zone: zoneToString(int(sa.ZoneId))}
	}
	return nil
}

func zoneToString(zone int) string {
	if zone == 0 {
		return ""
	}
	if ifi, err := net.InterfaceByIndex(zone); err == nil {
		return ifi.Name
	}
	return uitoa(uint(zone))
}

// Convert unsigned integer to decimal string.
func uitoa(val uint) string {
	if val == 0 { // avoid string allocation
		return "0"
	}
	var buf [20]byte // big enough for 64bit value base 10
	i := len(buf) - 1
	for val >= 10 {
		q := val / 10
		buf[i] = byte('0' + val - q*10)
		i--
		val = q
	}
	// val < 10
	buf[i] = byte('0' + val)
	return string(buf[i:])
}

func sotype2netUnix(sotype int) (nett string, err error) {
	switch sotype {
	case unix.SOCK_STREAM:
		nett = "unix"
	case unix.SOCK_DGRAM:
		nett = "unixgram"
	case unix.SOCK_SEQPACKET:
		nett = "unixpacket"
	default:
		err = net.UnknownNetworkError(strconv.Itoa(sotype))
	}
	return
}

func net2sotypeUnix(nett string) (sotype int, err error) {
	switch nett {
	case "unix":
		sotype = unix.SOCK_STREAM
	case "unixgram":
		sotype = unix.SOCK_DGRAM
	case "unixpacket":
		sotype = unix.SOCK_SEQPACKET
	default:
		err = net.UnknownNetworkError(nett)
	}
	return
}

func addrFunc(family, sotype int) func(unix.Sockaddr) net.Addr {
	switch family {
	case unix.AF_INET, unix.AF_INET6:
		switch sotype {
		case unix.SOCK_STREAM:
			return sockaddrToTCP
		case unix.SOCK_DGRAM:
			return sockaddrToUDP
		case unix.SOCK_RAW:
			return sockaddrToIP
		}
	case unix.AF_UNIX:
		switch sotype {
		case unix.SOCK_STREAM:
			return sockaddrToUnix
		case unix.SOCK_DGRAM:
			return sockaddrToUnixgram
		case unix.SOCK_SEQPACKET:
			return sockaddrToUnixpacket
		}
	}
	return func(unix.Sockaddr) net.Addr { return nil }
}

func isSameUnixAddr(a1, a2 net.Addr) bool {
	if a1.Network() != a2.Network() {
		return false
	}
	a1s := a1.String()
	a2s := a2.String()
	return a1s == a2s
}

func isSameIPAddr(a1, a2 net.Addr) bool {
	if a1.Network() != a2.Network() {
		return false
	}
	a1s := a1.String()
	a2s := a2.String()
	if a1s == a2s {
		return true
	}

	// This allows for ipv6 vs ipv4 local addresses to compare as equal. This
	// scenario is common when listening on localhost.
	const ipv6prefix = "[::]"
	a1s = strings.TrimPrefix(a1s, ipv6prefix)
	a2s = strings.TrimPrefix(a2s, ipv6prefix)
	const ipv4prefix = "0.0.0.0"
	a1s = strings.TrimPrefix(a1s, ipv4prefix)
	a2s = strings.TrimPrefix(a2s, ipv4prefix)
	return a1s == a2s
}
