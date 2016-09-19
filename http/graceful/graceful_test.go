package graceful

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"testing"
	"time"
)

func killMeSoon(s *Server, delay time.Duration) {
	if delay != 0 {
		time.Sleep(delay)
	}
	for {
		time.Sleep(time.Millisecond)
		success := s.ShutdownOK()
		if success {
			break
		}
	}
}

func TestServe(t *testing.T) {
	s := &Server{
		Server: &http.Server{},
	}

	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	go killMeSoon(s, 0)
	err = s.Serve(l)
	if err != nil {
		t.Fatal(err)
	}
}

func TestKeepaliveShutdown(t *testing.T) {

	mux := http.NewServeMux()
	mux.HandleFunc("/clientleaves", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "nice")
	})
	mux.HandleFunc("/servertimeout", func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "nice")
	})
	mux.HandleFunc("/longdownload", func(w http.ResponseWriter, req *http.Request) {
		time.Sleep(3 * time.Second)
		fmt.Fprintf(w, "finally")
	})

	s := &Server{
		Server: &http.Server{
			Handler: mux,
		},
		Timeout: time.Second,
	}

	for _, usecase := range []string{"clientleaves", "servertimeout", "longdownload"} {

		// make a listener
		l, err := net.Listen("tcp", "")
		if err != nil {
			t.Fatal(err)
		}

		addr := l.Addr()

		go s.Serve(l)

		tr := &http.Transport{}

		if usecase == "clientleaves" {
			tr.DisableKeepAlives = true
		}

		client := &http.Client{
			Transport: tr,
		}

		go killMeSoon(s, 1*time.Second)
		resp, err := client.Get("http://" + addr.String() + "/" + usecase)
		if err != nil {
			if usecase != "longdownload" {
				t.Fatal(err)
			}
		} else {
			if usecase == "longdownload" {
				t.Fatal("Connection should have been closed")
			}
			if resp.StatusCode != 200 {
				t.Fatalf("Wrong http response: %s", resp.Status)
			}
		}
		s.Wait()

		killed := s.ConnectionsKilled()
		switch usecase {
		case "clientleaves":
			fallthrough
		case "servertimeout":
			if killed != 0 {
				t.Fatalf("Zero connections should have been killed, but, got %d", killed)
			}
		case "longdownload":
			if killed != 1 {
				t.Fatalf("One connection should have been killed, but, got %d", killed)
			}
		}
	}
}

func ExampleServer_Serve() {
	s := &Server{
		Server: &http.Server{},
	}

	l, err := net.Listen("tcp", "")
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		time.Sleep(time.Second)
		s.Shutdown()
		s.Wait()
	}()

	err = s.Serve(l)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("OK")
	// Output:
	// OK
}
