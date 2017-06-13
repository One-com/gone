package accesslog

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"testing"
)

func TestLog(t *testing.T) {

	// make a random listener
	l, err := net.Listen("tcp", "")
	if err != nil {
		t.Fatal(err)
	}
	// remember where it listens.
	saddr := l.Addr()

	var elogbuf bytes.Buffer

	go serve(l, &elogbuf)

	client := &http.Client{}

	_, err = client.Get("http://" + saddr.String())
	if err != nil {
		t.Fatal(err)
	}

	elog := elogbuf.Bytes()

	if !bytes.HasPrefix(elog, []byte("Error writing access")) {
		t.Fatal("Handler didn't log accesslog error")
	}
}

var myhandler = http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
	rw.Write([]byte("hejsa\n"))
})

type badwriter struct{}

func (b badwriter) Write(p []byte) (n int, err error) {
	return 0, errors.New("Nope")
}

func serve(l net.Listener, elogbuf io.Writer) {
	h := NewDynamicLogHandler(myhandler, nil)
	s := http.Server{
		Handler:  h,
		ErrorLog: log.New(elogbuf, "", 0),
	}
	h.ToggleAccessLog(nil, badwriter{})

	s.Serve(l)
}
