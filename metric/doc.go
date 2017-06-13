/*
Package metric is a generic metric package for the standard metric types gauges/counters/timers/histograms. It ships with a statsd sink implementation, but can be extended with new sinks.

This implementation is aimed at being as fast as possible to not discourage metrics on values in hotpaths just because of locking overhead. This requires some client side buffering (and flusher go-routines) and, especially for the timer/histogram event type, a relatively large data structure to create a ring-buffer with mostly lock-free writes. (it uses condition variables for flushing).
This design is for the use case where you have a lot of timer/histogram metric events going to a few buckets.

The API is accessed via Client objects. (There's a default global client). The low latency and speed is achieved by creating permanent buffering metrics objects for each metric. Alternatively you can manually send ad-hoc metrics directly to the Clients configured sink. This allows for use cases where you don't have a small set of known metrics receiving many events, but rather many metrics receiving few evetns each.

The statsd sink also does client side buffering before sending UDP packages and is flushed when asked, or when running full. You can set the max size of the UDP datagrams created.
*/
package metric

