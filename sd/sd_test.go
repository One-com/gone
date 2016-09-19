package sd

import (
	"net/http"
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

	<- again

	l2,_, err := InheritNamedListener("mylistener")
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

}
