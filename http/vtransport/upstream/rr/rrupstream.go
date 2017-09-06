package rr

import (
	"errors"
	"github.com/One-com/gone/http/vtransport"
	"net/http"
	"net/url"
	"sync"
	"time"
)

type rrcontext struct {
	current *url.URL
	first   int
	idx     int
	retry   int
	wrap    int
	pinkey  string
}

func (rc *rrcontext) Target() (string, string) {
	return rc.current.Scheme, rc.current.Host
}

func (rc *rrcontext) Retries() int {
	return rc.retry
}

func (rc *rrcontext) Exhausted() int {
	return rc.wrap
}

// Cache is the interface needed to allow the RR upstream to remember
// where to send successful request for a period of time, (based on a cache key)
// to avoid moving requests too much around between backend servers.
type Cache interface {
	Set(key string, value int, ttl time.Duration)
	Get(key string) (value int)
	Delete(key string)
}

type target struct {
	*url.URL
	fails int
	when  time.Time
	down  bool
}

// PinKeyFunc is a function which, based on a request, returns a string as
// a routing key for the request, to allow the Round Robin upstream to cache
// the picked server for requests with that routing key.
type PinKeyFunc func(req *http.Request) string

type roundRobinUpstream struct {
	mu             sync.Mutex
	idx            int
	smu            sync.Mutex
	maxFails       int
	quarantineTime time.Duration
	targets        []target
	ctxPool        *sync.Pool
	pinKeyFunc     PinKeyFunc
	cache          Cache
	pinttl         time.Duration
	evcbf          func(Event)
}

// Event reports named event with the upstream pool via a callback function.
// Currently named events: "quarantine", "retrying"
type Event struct {
	Name string
	Target *url.URL
}

// RROption is an option function configuring a Round Robin VirtualUpstream
type RROption func(*roundRobinUpstream)

// Targets sets the backend URLs for the Round Robin upstream
func Targets(urls ...*url.URL) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		if rr.targets == nil {
			rr.targets = make([]target, 0, 1)
		}
		for _, u := range urls {
			rr.targets = append(rr.targets, target{URL: u})
		}
	})
}

// MaxFails configures how many times a backend server is allowed to fail
// before being put into quarantine.
func MaxFails(m int) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		rr.maxFails = m
	})
}

// Quarantine configures the quarantine period for failed servers.
func Quarantine(d time.Duration) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		rr.quarantineTime = d
	})
}

// PinRequestsWith will - if provided a cache implementation - allow the Round Robin upstream
// to remember where it send requests with a similar routing key and keep sending them to that
// server. This prevents picking a new server for every single related request.
// The provided PinkeyFunc is called on requests to determine the routing key.
func PinRequestsWith(cache Cache, ttl time.Duration, f PinKeyFunc) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		rr.pinKeyFunc = f
		rr.cache = cache
		rr.pinttl = ttl
	})
}

// EventCallback can be set to notify the application about changes to the upstream
// pool. This can be used for logging when a server is quarantined.
func EventCallback(f func(Event)) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		rr.evcbf = f
	})
}

// NewRoundRobinUpstream returns a Round Robin VirtualUpstream configured
// with the provided options.
func NewRoundRobinUpstream(opts ...RROption) (*roundRobinUpstream, error) {
	ret := &roundRobinUpstream{
		ctxPool: &sync.Pool{
			New: func() interface{} { return new(rrcontext) },
		},
	}
	for _, o := range opts {
		o(ret)
	}
	if len(ret.targets) == 0 {
		return nil, errors.New("No targets")
	}
	return ret, nil
}

// ReleaseContext implements the VirtualUpstream interface.
func (u *roundRobinUpstream) ReleaseContext(inctx vtransport.RoundTripContext) {
	if inctx != nil {
		ctx := inctx.(*rrcontext)
		u.ctxPool.Put(ctx)
	}
}

// Update implements the VirtualUpstream interface.
func (u *roundRobinUpstream) Update(inctx vtransport.RoundTripContext, err error) {
	u.smu.Lock()
	defer u.smu.Unlock()
	ctx := inctx.(*rrcontext)
	if err == nil {
		if ctx.pinkey != "" {
			u.cache.Set(ctx.pinkey, ctx.idx, u.pinttl)
		}
		// reset fail counter
		u.targets[ctx.idx].fails = 0
		return
	}
	if ctx.pinkey != "" {
		u.cache.Delete(ctx.pinkey)
	}
	u.targets[ctx.idx].fails++
	fails := u.targets[ctx.idx].fails
	if u.maxFails != 0 && fails >= u.maxFails {
		// mark server down
		if u.evcbf != nil {
			u.evcbf(Event{Name: "quarantine", Target:  u.targets[ctx.idx].URL})
		}
		u.targets[ctx.idx].down = true
		u.targets[ctx.idx].when = time.Now()
	}
}

func (u *roundRobinUpstream) step(in int) (out int) {
	in++
	if in >= len(u.targets) {
		in = 0
	}
	return in
}

func (u *roundRobinUpstream) newRoundTripContext(first int) *rrcontext {
	ctx := u.ctxPool.Get().(*rrcontext)
	*ctx = rrcontext{first: first} // zero out
	return ctx
}

// NextTarget implements the VirtualUpstream interface.
func (u *roundRobinUpstream) NextTarget(req *http.Request, in vtransport.RoundTripContext) (out vtransport.RoundTripContext, err error) {

	var start int // where this search for a next target started
	var ctx *rrcontext
	if in == nil {
		var pinkey string
		if u.pinKeyFunc != nil {
			pinkey = u.pinKeyFunc(req)
			start = u.cache.Get(pinkey)
		}
		if u.pinKeyFunc == nil || start == -1 {
			// Pick the next overall server as starting point for search
			u.mu.Lock()
			start = u.step(u.idx)
			u.idx = start
			u.mu.Unlock()
		}
		ctx = u.newRoundTripContext(start)
		ctx.pinkey = pinkey
	} else {
		ctx = in.(*rrcontext)
		ctx.retry++

		// Pick the next server from the context as starting point
		start = u.step(ctx.idx)

		// If the next server is the first we tried - note exhaustion
		if start == ctx.first {
			ctx.wrap++
		}
	}

	// Start search to find a healthy server
	next := start
	u.smu.Lock()
	for {
		if !u.targets[next].down {
			// We found a healthy server
			break
		}

		if time.Now().Sub(u.targets[next].when) > u.quarantineTime {
			// We found a sick server having done its Quarantine
			if u.evcbf != nil {
				u.evcbf(Event{Name: "retrying", Target:  u.targets[next].URL})
			}
			u.targets[next].down = false
			break
		}

		// Try next server in pool (Round Robin style)
		next = u.step(next)
		if next == start {
			// We've tried all servers
			// But don't fail without trying something
			ctx.wrap++
		}
	}
	u.smu.Unlock()

	ctx.idx = next
	ctx.current = u.targets[next].URL
	out = ctx

	return
}
