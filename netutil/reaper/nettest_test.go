package reaper_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"gitlab.one.com/go/netutil/reaper"
	"golang.org/x/net/nettest"
)

// running test-suite with race falg might help a bit
// GO_PKG_BASE:/netutil/reaper$ go test -race
func testReaperConn(t *testing.T, useDefaultListener, useDefaultDialer bool, timeout time.Duration) {
	reaperInterval := time.Duration((timeout.Nanoseconds() / 2)) * time.Nanosecond
	tests := []struct{ name, network string }{
		{"TCP", "tcp"},
		{"TCP6", "tcp6"},
		{"UnixPipe", "unix"},
		{"UnixPacketPipe", "unixpacket"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !testableNetwork(tt.network) {
				t.Skipf("not supported on %s", runtime.GOOS)
			}

			mp := func() (c1, c2 net.Conn, stop func(), err error) {
				var ln net.Listener
				l, err := newLocalListener(tt.network)
				if err != nil {
					return nil, nil, nil, err
				}
				if useDefaultListener {
					ln = l
				} else {
					ln = reaper.NewIOActivityTimeoutListener(l, timeout, reaperInterval)
				}

				// Start a connection between two endpoints.
				var err1, err2 error
				done := make(chan bool)
				go func() {
					c2, err2 = ln.Accept()
					close(done)
				}()
				dialer := &net.Dialer{
					Timeout:   time.Minute,
					KeepAlive: time.Minute,
				}
				if useDefaultDialer {
					c1, err1 = dialer.Dial(ln.Addr().Network(), ln.Addr().String())
				} else {
					timeoutDialer := reaper.NewIOActivityTimeoutDialer(dialer, timeout, reaperInterval, true)
					c1, err1 = timeoutDialer.Dial(ln.Addr().Network(), ln.Addr().String())
				}
				<-done

				stop = func() {
					if err1 == nil {
						c1.Close()
					}
					if err2 == nil {
						c2.Close()
					}
					ln.Close()
					switch tt.network {
					case "unix", "unixpacket":
						os.Remove(ln.Addr().String())
					}
				}

				switch {
				case err1 != nil:
					stop()
					return nil, nil, nil, err1
				case err2 != nil:
					stop()
					return nil, nil, nil, err2
				default:
					return c1, c2, stop, nil
				}
			}

			nettest.TestConn(t, mp)
		})
	}
}

func TestIOActivityTimeout(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"PartialSecond", time.Millisecond * 500},
		{"Second", time.Second},
		{"TenSecond", time.Second * 10},
	}

	for _, testDetail := range tests {
		testReaperConn(t, true, false, testDetail.timeout)
		testReaperConn(t, false, true, testDetail.timeout)
		testReaperConn(t, false, false, testDetail.timeout)
	}
}

// imports from golang.org/x/net/internal/nettest

var (
	supportsIPv4 bool
	supportsIPv6 bool
)

func init() {
	if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err == nil {
		ln.Close()
		supportsIPv4 = true
	}
	if ln, err := net.Listen("tcp6", "[::1]:0"); err == nil {
		ln.Close()
		supportsIPv6 = true
	}
}

// NewLocalListener returns a listener which listens to a loopback IP
// address or local file system path.
// Network must be "tcp", "tcp4", "tcp6", "unix" or "unixpacket".
func newLocalListener(network string) (net.Listener, error) {
	switch network {
	case "tcp":
		if supportsIPv4 {
			if ln, err := net.Listen("tcp4", "127.0.0.1:0"); err == nil {
				return ln, nil
			}
		}
		if supportsIPv6 {
			return net.Listen("tcp6", "[::1]:0")
		}
	case "tcp4":
		if supportsIPv4 {
			return net.Listen("tcp4", "127.0.0.1:0")
		}
	case "tcp6":
		if supportsIPv6 {
			return net.Listen("tcp6", "[::1]:0")
		}
	case "unix", "unixpacket":
		return net.Listen(network, localPath())
	}
	return nil, fmt.Errorf("%s is not supported", network)
}

func localPath() string {
	f, err := ioutil.TempFile("", "nettest")
	if err != nil {
		panic(err)
	}
	path := f.Name()
	f.Close()
	os.Remove(path)
	return path
}

// TestableNetwork reports whether network is testable on the current
// platform configuration.
func testableNetwork(network string) bool {
	// This is based on logic from standard library's
	// net/platform_test.go.
	switch network {
	case "unix", "unixgram":
		switch runtime.GOOS {
		case "android", "nacl", "plan9", "windows":
			return false
		}
		if runtime.GOOS == "darwin" && (runtime.GOARCH == "arm" || runtime.GOARCH == "arm64") {
			return false
		}
	case "unixpacket":
		switch runtime.GOOS {
		case "android", "darwin", "freebsd", "nacl", "plan9", "windows":
			return false
		}
	}
	return true
}
