# gone/http/vtransport

A golang library providing a http.RoundTripper implementation which will send a HTTP request
to a URL picked by a VirtualUpstream implementation - maybe retrying the request on failure.

A reasonable functional RoundRobin implementation of VirtualUpstream is provided.

## Example

```go
	b1, err := url.Parse("http://localhost:8001")
	b2, err := url.Parse("http://localhost:8002")

	upstream, err := rr.NewRoundRobinUpstream(
		rr.Targets(b2, b1),
		rr.MaxFails(2),
		rr.Quarantine(time.Duration(2)*time.Second))

	tr := &vtransport.VirtualTransport{
		Transport: http.Transport{
			DisableKeepAlives: true,
		},
		RetryPolicy: vtransport.NoRetry,
		Upstreams:   map[string]vtransport.VirtualUpstream{"backend": upstream},
	}
	client := &http.Client{Transport: tr}
```

