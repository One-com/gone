package vtransport

import (
	"fmt"
	"net"
	"context"
	"net/http"
)

// RoundTripContext is an object maintained by the virtual upstream implementation
// which is opaque to the VirtualTransport apart from the methods specified.
// It should carry information enough for the VirtualUpstream to do any book keeping
// needed when Update() is called with the result of sending the request to the current
// target host.
type RoundTripContext interface {
	// Target should return the URL scheme and the host for the
	// current backend target
	Target() (scheme, host string)
	// Retries should return the number of retries which has been made
	Retries() int
	// Exhausted should return how many times retries have led to all
	// relevant backend servers being tried.
	Exhausted() int
}

// VirtualUpstream is an implementation of a logical upstream, which may consist of
// one or more actual backend network hosts, implementing the selection process to
// chose the actual host for a HTTP request.
type VirtualUpstream interface {
	// NextTarget is called by the VirtualTransport to advance the context
	// to the next backend server. The first call to NextTarget will be done
	// with a nil context and the VirtualUpstream implemetation should return
	// the first context. The context will be released again with ReleaseContext()
	NextTarget(req *http.Request, context RoundTripContext) (RoundTripContext, error)
	// Update notified the VirtualUpstream of the result from sending the HTTP request
	// to the current Target host.
	Update(context RoundTripContext, err error)
	// ReleaseContext tells the VirtualUpstream that the context is no longer needed
	ReleaseContext(RoundTripContext)
}

// RetryPolicy is a function deciding whether the request should be retried
// after a non-nil error from a call to the underlying RoundTripper
type RetryPolicy func(req *http.Request, err error, ctx RoundTripContext) bool

// NoRetry never retries. Equivalient to a nil RetryPolicy
var NoRetry = RetryPolicy(
	func(req *http.Request, err error, ctx RoundTripContext) bool {
		return false
	})

// http://www.iana.org/assignments/http-methods/http-methods.xhtml
func isIdempotent(method string) bool {
	switch method {
	case "GET", "HEAD":
		// RFC7231, 4.2.1
		return true
	case "PUT", "DELETE":
		// RFC7231, 4.2.2
		return true
	case "PROPFIND":
		// RFC4918,a 9.1
		return true
	case "PROPPATCH":
		// RFC4918,a 9.2
		return true
	case "REPORT":
		return true
	case "MKCOL":
		// RFC4918,a 9.3
		return true
	case "COPY":
		// RFC4918,a 9.8
		return true
	case "MOVE":
		// RFC4918,a 9.9
		return true
	case "UNLOCK":
		// RFC4918,a 9.11
		return true
	case "OPTIONS", "TRACE":
		// RFC7231, 4.2.1
		return true
	}
	return false
}

// Retries creates a RetryPolicy which will at most retry the request "retries" times.
// and at most repeat the backend host pool "maxRepeat" times.
// Per default if will only retry idempotent requests
func Retries(retries int, maxRepeat int, nonIdempotent bool) RetryPolicy {
	return func(req *http.Request, err error, ctx RoundTripContext) bool {
		if ctx.Retries() >= retries {
			return false
		}
		if ctx.Exhausted() > maxRepeat {
			return false
		}
		if nonIdempotent || isIdempotent(req.Method) {
			return true
		}
		if ne, ok := err.(*net.OpError); ok {
			if ne.Op == "dial" {
				return true
			}
		}
		return false
	}
}

// VirtualTransport acts as a replacement for the stdlib http.Transport but uses a set of named
// VirtualUpstream implementations to do the RoundTripper functionality if the URL scheme is "vt" and will consult the
// provided RetryPolicy to decide whether to retry HTTP requests which fails.
type VirtualTransport struct {
	*http.Transport
	Upstreams   map[string]VirtualUpstream
	RetryPolicy RetryPolicy
}

// RoundTrip implements the http.RoundTripper interface.
func (vt *VirtualTransport) RoundTrip(req *http.Request) (resp *http.Response, err error) {

	if req.URL.Scheme != "vt" {
		return vt.Transport.RoundTrip(req)
	}

	var up VirtualUpstream

	upname := req.URL.Host

	up = vt.Upstreams[upname]
	if up == nil {
		return nil, fmt.Errorf("No such upstream %s", upname)
	}

	bodywrapper := &deferCloseBody{
		ReadCloser: req.Body,
	}
	req.Body = bodywrapper

	var ctx RoundTripContext
RETRIES:
	for {
		ctx, err = up.NextTarget(req, ctx)
		if err != nil {
			up.ReleaseContext(ctx)
			return
		}

		req.URL.Scheme, req.URL.Host = ctx.Target()

		resp, err = vt.Transport.RoundTrip(req)
		var uerr error
		if err == context.Canceled {
			// Don't tell the upstream about RoundTrip errors
			// if it was actually the client canceling the request
			// It is not to blame
			uerr = nil
		} else {
			uerr = err
		}
		up.Update(ctx, uerr)
		// We are satisfied by non-error or client cancellation
		if uerr == nil { // success return response.
			up.ReleaseContext(ctx)
			// but return real error so caller know client canceled
			return resp, err
		}
		if vt.RetryPolicy == nil || !vt.RetryPolicy(req, err, ctx) {
			break RETRIES
		}
		if !bodywrapper.CanRetry() {
			break RETRIES
		}
	}
	bodywrapper.CloseIfNeeded()
	up.ReleaseContext(ctx)
	return resp, err
}
