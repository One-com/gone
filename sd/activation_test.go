package sd_test

import (
	"os"
	"os/exec"
	"net"
	"fmt"
	"bufio"
	"time"
	"testing"
	"syscall"
)

var systemd_activate = "/lib/systemd/systemd-activate"

var listen_address = "127.0.0.1:54321"

func TestSocketActivation(t *testing.T) {

	cmd := exec.Command(systemd_activate , "-E", "LISTEN_PID_IGNORE=1", "--listen=" + listen_address, "go", "run", "testbin/sdtest.go" )

	err := cmd.Start()
	if err != nil {
		if e2, ok := err.(*os.PathError); ok {
			if e2.Err == syscall.ENOENT {
				t.Skip("systemd-activate not found - skipping")
			}
		}
		t.Fatalf("%#v %s", err, err.Error())
	}

	defer func() {
		cmd.Process.Signal(syscall.SIGTERM)
		cmd.Wait()
	}()

	time.Sleep(time.Millisecond)

	// Connection and readback
	conn, err := net.Dial("tcp", listen_address)
	if err != nil {
		t.Fatalf(err.Error())
	}
	data := "Hello\n"
	fmt.Fprintf(conn, data)
	status, err := bufio.NewReader(conn).ReadString('\n')
	if err != nil {
		t.Fatalf(err.Error())
	}
	if status != data  {
		t.Fatalf("Din't get back test data. Expected <%s>, got <%s>", data, status)
	}
	fmt.Fprintln(conn, "quit")
}
