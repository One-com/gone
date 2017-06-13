package sd

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"testing"
)

func TestFileWith(t *testing.T) {

	s := &http.Server{}

	// make a listener
	l, err := NamedListenTCP("mylistener", "tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	addr := l.Addr()

	again := make(chan struct{})
	go func() {
		s.Serve(l)
		Reset()
		close(again)
	}()

	l.Close()

	<-again

	l2, _, err := InheritNamedListener("mylistener")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		s.Serve(l2)
	}()

	client := &http.Client{}

	resp, err := client.Get("http://" + addr.String() + "/")

	if err != nil || resp.StatusCode != 404 {
		t.Fatal("Failed using socket")
	}

	l2.Close()
	Reset()
	Cleanup()
}

//func TestPid(t *testing.T) {
//	fmt.Printf("PID %d\n", os.Getpid())
//}

func countOpenFds() (int, error) {

	procdir := fmt.Sprintf("/proc/%d/fd", os.Getpid())
	dir, err := os.Open(procdir)
	if err != nil {
		return 0, err
	}
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return 0, err
	}
	//fmt.Println("my",dir.Fd(),names)
	dir.Close()

	return len(names), nil
}

func TestExport(t *testing.T) {

	s := &http.Server{}

	// make a native listener - and export it to gone/sd
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = Export("mylistener", l)
	if err != nil {
		t.Fatal(err)
	}

	addr := l.Addr()

	again := make(chan struct{})
	go func() {
		s.Serve(l)
		close(again) // allow main test routine to proceed.
	}()

	if len(_availablefds()) != 0 || len(_activefds()) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l.Close() // provoke Serve() to exit

	<-again
	Reset() // allow using active file descriptors again

	if len(_availablefds()) != 1 || len(fdState.active) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l2, _, err := InheritNamedListener("mylistener")
	if err != nil {
		t.Fatal(err)
	}

	if l2 == nil {
		t.Fatal("Could not reclaim listener")
	}

	if len(_availablefds()) != 0 || len(fdState.active) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	go func() {
		s.Serve(l2)
	}()

	client := &http.Client{}

	resp, err := client.Get("http://" + addr.String() + "/")

	if err != nil || resp.StatusCode != 404 {
		t.Fatal("Failed using socket")
	}

	l2.Close()

	Forget("mylistener")

	if len(_availablefds()) != 0 || len(fdState.active) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(_activefds()))
	}

	l3, _, err := InheritNamedListener("mylistener")
	if err != nil {
		t.Fatal(err)
	}

	if l3 != nil {
		t.Fatal("Listener should be closed")
	}

	Reset()
	Cleanup()
}

func TestForget(t *testing.T) {

	// make a native listener - and export it to gone/sd
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = Export("mylistener2", l)
	if err != nil {
		t.Fatal(err)
	}

	if len(_availablefds()) != 0 || len(_activefds()) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l.Close() // provoke Serve() to exit

	Reset() // allow using active file descriptors again

	if len(_availablefds()) != 1 || len(_activefds()) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l2, l2name, err := InheritNamedListener("", IsTCPListener(nil))
	if err != nil {
		t.Fatal(err)
	}

	if l2 == nil {
		t.Fatal("Could not reclaim listener")
	}

	if len(_availablefds()) != 0 || len(_activefds()) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l2.Close()

	Forget(l2)

	Reset()

	if len(_availablefds()) != 0 || len(_activefds()) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(_activefds()))
	}

	l3, _, err := InheritNamedListener(l2name)
	if err != nil {
		t.Fatal(err)
	}

	if l3 != nil {
		t.Fatal("Listener should be closed")
	}

	Reset()
	Cleanup()
}

func TestListenTCP(t *testing.T) {
	// make a native listener - and export it to gone/sd
	l, err := net.ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	err = Export("mylistener2", l)
	if err != nil {
		t.Fatal(err)
	}

	if len(_availablefds()) != 0 || len(_activefds()) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l.Close() // provoke Serve() to exit

	Reset() // allow using active file descriptors again

	if len(_availablefds()) != 1 || len(_activefds()) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l2, err := ListenTCP("tcp", nil)
	if err != nil {
		t.Fatal(err)
	}

	if l2 == nil {
		t.Fatal("Could not reclaim listener")
	}

	if len(_availablefds()) != 0 || len(_activefds()) != 1 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(fdState.active))
	}

	l2.Close()

	Forget(l2)

	Reset()

	if len(_availablefds()) != 0 || len(_activefds()) != 0 {
		t.Fatalf("Number of open FDs: %d/%d\n", len(_availablefds()), len(_activefds()))
	}

	Cleanup()

}
