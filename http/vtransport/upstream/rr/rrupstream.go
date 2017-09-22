package rr

import (
	"fmt"
	"context"
	"errors"
	"github.com/One-com/gone/http/vtransport"
	"net/http"
	"net/url"
	"sync"
	"time"
	"math/rand"
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
	// Get must return the value set, or negative if not found
	Get(key string) (value int)
	Delete(key string)
}

type target struct {
	*url.URL
	fails int       // number of fails recorded for this target
	when  time.Time // time of last fail
	down  bool      // marked as down, waiting for quarantine to expire
}

// PinKeyFunc is a function which, based on a request, returns a string as
// a routing key for the request, to allow the Round Robin upstream to cache
// the picked server for requests with that routing key.
type PinKeyFunc func(req *http.Request) string

type roundRobinUpstream struct {
	mu             sync.Mutex
	id             string
	idx            int
	smu            sync.Mutex
	maxFails       int
	burstGrace     time.Duration
	quarantineTime time.Duration
	targets        []target
	ctxPool        *sync.Pool
	pinKeyFunc     PinKeyFunc
	cache          Cache
	pinttl         time.Duration
	configured     chan struct{} // closed when all options have been applied
	evcbf          func(Event)
}

// Event reports named event with the upstream pool via a callback function.
// Currently named events: "quarantine", "retrying"
type Event struct {
	Name string
	Target *url.URL
	Err error
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

// HealthCheck configures a periodic health check callback.
// The provided checkfunc will be called with the URL of the backend target to check.
// checkfunc can perform any activity it wish and returning anything but a nil error,
// will mark the backend as down.
// The simple check would be to adjust the target URL to add a Request-URI and perform
// an HTTP request to verify it's up.
// If this check fails it will keep the target from being picked by the Round Robin algorithm until
// the next health check.
// The health check will run when startfunc is called until the context is canceled.
// This option will return nil if interval is zero.
func HealthCheck(interval time.Duration, checkfunc func(*url.URL) error) (option RROption, startfunc func(context.Context) error){

	var upstream *roundRobinUpstream

	if interval == 0 {
		return // provoke an error early by returning nil RROption
	}

	proceed := make(chan struct{})

	option = RROption(func(rr *roundRobinUpstream) {
		upstream = rr
		close(proceed)
	})

	startfunc = func(ctx context.Context) error {

		// don't proceed until we know which upstream to monitor
		select {
		case <-ctx.Done():
			close(proceed)
			return nil
		case <-proceed:
		}

		// We now know the upstream, but we're not sure it's fully configured.
		select {
		case <-ctx.Done():
			return nil
		case <-upstream.configured:
			// don't monitor empty upstreams
			if len(upstream.targets) == 0 {
				return nil
			}
		}

		// Now we have a fully configured upstream, start monitoring
		ticker := time.NewTicker(interval)

		HEALTHLOOP: for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				break HEALTHLOOP
			case <-ticker.C:
				// Run through all the backends, generate the URL, do the req.,
				// check expect and update target status
				for i := range upstream.targets {
					var u url.URL
					var ev Event
					// copy the URL to local value.
					u.Scheme = upstream.targets[i].URL.Scheme
					u.Host   = upstream.targets[i].URL.Host

					err := checkfunc(&u) // callee is allowed to modify u
					if err != nil {
						// Backend target has failed health check
						// Mark it down now
						upstream.mu.Lock()
						upstream.targets[i].down = true
						upstream.targets[i].when = time.Now().Add(interval)
						upstream.mu.Unlock()
						ev = Event{Name: "healthfail", Target: &u, Err: err}
					} else {
						// Yeah - it's alive. Put it back in the pool
						// by making sure reasonable lower than maxFails
						upstream.mu.Lock()
						upstream.targets[i].down = false
						upstream.targets[i].fails = upstream.maxFails / 2
						upstream.mu.Unlock()
					}
					if upstream.evcbf != nil && ev.Name != "" {
						upstream.evcbf(ev)
					}
				}
			}
		}
		return nil
	}

	return
}

// BurstFailGrace sets a duration for counting multiple errors as one.
// This prevents backend servers which in case of an isolated incidence
// will kill multiple HTTP request from reaching the MaxFails limit immediately.
// Fails not counted due to BurstFailGrace do not count as successes though.
func BurstFailGrace(t time.Duration) RROption {
	return RROption(func(rr *roundRobinUpstream) {
		rr.burstGrace = t
	})
}

// MaxFails configures how many times a backend server is allowed to fail
// before being put into quarantine. As fails are recorded for a backend
// it's picked less often with a chance proportional to MaxFails.
// A MaxFail of zero (default), means don't quarantine backend targets and don't
// weigh targets with fails lower when selecting next target.
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
	ret.id = fmt.Sprintf("%p", ret)
	ret.configured = make(chan struct{})
	for _, o := range opts {
		o(ret)
	}
	close(ret.configured)
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

	ctx := inctx.(*rrcontext)
	if err == nil {
		if ctx.pinkey != "" {
			u.cache.Set(ctx.pinkey, ctx.idx, u.pinttl)
		}
		// lower the fail counter
		if u.targets[ctx.idx].fails > 0 {
			u.targets[ctx.idx].fails--
		}
		u.smu.Unlock()
		return
	}

	// An error reported, we need to decide on punishing the backend.

	// First removing any pinning.
	if ctx.pinkey != "" {
		u.cache.Delete(ctx.pinkey)
	}

	var ev Event // possible event to report

	// If we are counting fails to decide target status:
	if u.maxFails != 0  {
		tnow := time.Now()

		// Count the error if sufficiently long time since last error to not be a burst
		if tnow.Sub(u.targets[ctx.idx].when) > u.burstGrace {
			u.targets[ctx.idx].fails++
			u.targets[ctx.idx].when = tnow

			// Decide whether to completely quarantine this backend target
			fails := u.targets[ctx.idx].fails
			if fails >= u.maxFails {
				// mark server down
				ev = Event{Name: "quarantine", Target:  u.targets[ctx.idx].URL}
				u.targets[ctx.idx].down = true
			}
		} else {
			// We ignore this fail as a part of a burst.
			// Don't record the timestamp in order to not keep pushing the grace period.
			ev = Event{Name: "burst", Target:  u.targets[ctx.idx].URL}
		}
	}
	u.smu.Unlock()

	// Report a quarantine event if any - don't do this under mutex lock
	if u.evcbf != nil && ev.Name != "" {
		u.evcbf(ev)
	}
}

// simply doing an increment module number of target hosts
func (u *roundRobinUpstream) step(in int) (int) {
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
			if pinkey != "" {
				pinkey = u.id + pinkey // make pinkey private to this upstream
				start = u.cache.Get(pinkey)
			} else {
				start = -1
			}
		}
		if u.pinKeyFunc == nil || start < 0 {
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
	var ev Event
	u.smu.Lock()
	for {
		if !u.targets[next].down {
			// We found a healthy server
			fails := u.targets[next].fails
			if u.maxFails == 0 {
				break // Don't care about the number of fails - use the target
			} else {
				// - consider fail/maxfail ratio to decide whether to use it
				if fails == 0 || fails <= rand.Intn(u.maxFails) {
					// use the target, - should always use for fails==0
					break
				}
			}
			// Else we will try the next target
		} else {
			// The target is down, don't let it in unless quarantine time has been exceeded.
			if time.Now().Sub(u.targets[next].when) > u.quarantineTime {
				// We found a sick server having done its Quarantine
				ev = Event{Name: "retrying", Target:  u.targets[next].URL}
				u.targets[next].down = false
				break
			}
		}

		// Try next server in pool (Round Robin style)
		next = u.step(next)
		if next == start {
			// We've tried all servers
			// But don't fail without trying something
			ctx.wrap++
			break
		}
	}
	u.smu.Unlock()

	// report events - don't do this under mutex lock
	if u.evcbf != nil && ev.Name != "" {
		u.evcbf(ev)
	}

	ctx.idx = next
	ctx.current = u.targets[next].URL
	out = ctx

	return
}
