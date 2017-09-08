package vtransport_test

import (
	"github.com/One-com/gone/http/vtransport"
	"github.com/One-com/gone/http/vtransport/upstream/rr"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"testing"
	"time"
)

func makeMyHandler(server string) func(w http.ResponseWriter, r *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		var str string
		var i int
		var err error
		r.ParseForm()
		if str = r.Form.Get("val"); str == "" {
			http.Error(w, "Missing val", http.StatusBadRequest)
			return
		}
		if i, err = strconv.Atoi(str); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		log.Println("Req:", str, server)
		io.WriteString(w, strconv.Itoa(i))
	}
}

type cacheEntry struct {
	v int
	t time.Time
	x time.Duration
}

type backendCache struct {
	mu sync.Mutex
	c  map[string]*cacheEntry
}

func (c *backendCache) Set(key string, value int, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := &cacheEntry{v: value, t: time.Now(), x: ttl}
	c.c[key] = e
}
func (c *backendCache) Get(key string) (value int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, exists := c.c[key]
	if !exists {
		return -1
	}
	tooold := time.Now().Before(e.t.Add(e.x))
	if tooold {
		return -1
	}
	return e.v

}
func (c *backendCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.c, key)
}

func clientAbuse(t *testing.T) {

	b1, err := url.Parse("http://localhost:8001")
	if err != nil {
		log.Println(err)
		return
	}

	b2, err := url.Parse("http://localhost:8002")
	if err != nil {
		log.Println(err)
		return
	}

	cache := &backendCache{c: make(map[string]*cacheEntry)}

	upstream, err := rr.NewRoundRobinUpstream(
		rr.Targets(b2, b1),
		rr.PinRequestsWith(cache, time.Duration(10)*time.Second,
			rr.PinKeyFunc(func(req *http.Request) string {
				return "myroutingkey"
			})),
		rr.MaxFails(2),
		rr.Quarantine(time.Duration(2)*time.Second))
	if err != nil {
		log.Fatalf("ARG")
	}

	tr := &vtransport.VirtualTransport{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
		RetryPolicy: vtransport.Retries(3, 0, true),
		Upstreams:   map[string]vtransport.VirtualUpstream{"backend": upstream},
	}
	client := &http.Client{Transport: tr}

	for val := 0; val <= 10; val++ {
		var i int
		v := url.Values{}
		v.Set("val", strconv.Itoa(val))
		resp, err := client.PostForm("vt://backend/", v)
		if err != nil {
			log.Println("Err: ", err.Error())
			fail_mu.Lock()
			if !fail_expected {
				t.Fatal("Failure not expected")
			} else {
				break
			}
			fail_mu.Unlock()
		}
		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Fatal("Body read error")
		}
		body := string(data)
		if i, err = strconv.Atoi(body); err != nil {
			t.Fatalf("Bad answer: %s", body)

		}
		if i != val {
			t.Fatalf("Bad val %d", i)
		}
		resp.Body.Close()
		time.Sleep(time.Duration(1) * time.Second)

	}
}

func tryServe(s *http.Server, tl net.Listener) {
	err := s.Serve(tl)
	if err != nil {
		log.Println("Exited", err)
	}
}

func listenerFor(s *http.Server) net.Listener {
	addr := s.Addr

	tcpaddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}

	tl, err := net.ListenTCP("tcp", tcpaddr)
	if err != nil {
		log.Fatal(err)
	}
	return tl
}

var fail_mu sync.Mutex
var fail_expected bool

func TestBackendFail(t *testing.T) {

	server1 := &http.Server{Addr: "localhost:8001", Handler: http.HandlerFunc(makeMyHandler("S1"))}
	server2 := &http.Server{Addr: "localhost:8002", Handler: http.HandlerFunc(makeMyHandler("S2"))}

	l1 := listenerFor(server1)
	l2 := listenerFor(server2)

	go tryServe(server1, l1)
	go tryServe(server2, l2)

	go clientAbuse(t)

	time.Sleep(time.Duration(3) * time.Second)
	l1.Close()

	time.Sleep(time.Duration(3) * time.Second)
	fail_mu.Lock()
	fail_expected = true
	fail_mu.Unlock()
	l2.Close()

	time.Sleep(time.Second)
}
