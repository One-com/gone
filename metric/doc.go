/*
Package metric is a generic metric package for the standard metric types gauges/counters/timers/histograms. It ships with a statsd sink implementation, but can be extended with new sinks.

This implementation is aimed at being as fast as possible to not discourage metrics on values in hotpaths just because of locking overhead. This requires some client side buffering (and flusher go-routines) and, especially for the timer/histogram event type, a relatively large data structure to create a ring-buffer with mostly lock-free writes. (it uses condition variables for flushing).
This design is for the use case where you have a lot of timer/histogram metric events going to a few buckets.

The API consists of 3 main types of objects:

   * Meter - the object doing the actual measuring. (possibly buffering measurements)
   * Sink  - an object to which Meter readings can be sent. Sinks can have internal buffering which can be flushed
   * Client - an object responsible for sending Meter readings to an assigned Sink, possibly at configured flushing intervals.

The API supports 3 approaches to maintaining your metrics:

Ad-hoc

Ad-hoc generation of metric events without a Meter object (explicitly flushing the Client sink):
This is a relative slow, but simple way to generate metric data.
This allows for use cases where you don't have a small set of known metrics receiving many events, but rather many metrics receiving few events each:

   metric.SetDefaultSink(sink)
   metric.AdhocGauge("gauge", 17, true)

Manually

Manually making readings of a meter directly to a Sink. (use this only if strictly needed)

   gauge := metric.NewGauge("gauge")
   gauge.Set(17)
   gauge.FlushReading(sink)
   sink.Flush()

Managed

Registring a buffered Meter object with a client. This is the fastest method.
The low latency API is accessed via Client objects. (There's a default global client). The speed is achieved by creating permanent buffering metrics objects for each metric.

   client := metric.NewClient(sink, metric.FlushInterval(time.Second))
   gauge := client.RegisterGauge("gauge")
   gauge.Set(17)


The statsd sink also does client side buffering before sending UDP packages and is flushed when asked, or when running full. You can set the max size of the UDP datagrams created.
*/
package metric
