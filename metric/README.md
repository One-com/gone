# gone/metric

Fast Golang metrics library [![GoDoc](https://godoc.org/github.com/one-com/gone/metric?status.svg)](https://godoc.org/github.com/one-com/gone/metric) [![GoReportCard](https://goreportcard.com/badge/github.com/One-com/gone)](https://goreportcard.com/report/github.com/One-com/gone/metric) [Coverage](http://gocover.io/github.com/One-com/gone/metric)

Package gone/metric is an expandable library for metrics.
Initially only a sink for sending data to statsd is implemented.

The design goals:

* Basic statsd metric types (gauge, counter, timer)
* Client side buffered if needed.
* Fast

Timer and Histogram is basically the same except for the argument type.

Counter is reset to zero on each flush. Gauges are not.

## Permanent and ad-hoc meters

The library provides APIs for generating metrics events.

If you need Permanent meters (gauges/counters/timers...) which will be updated often with lots of data and want to use the lock free client side buffering to make it really fast, you can create explicit gauge/counter/timer/histogram objects using the client.RegisterXXX() methods.

If you, on the other hand, rarely record a metric event more than once, but have a lot of different named events, you can bypass the buffering data structures and send the metric event directly to the sink by using client.AdhocGauge(), client.AdhocCount(), client.AdhocTime() ...

Such direct recording of metrics events will still (depending on sink implementation) be bufferend in the protocol layer - unless you request an explicit flush.

This is maybe best illustrated with a Counter example:

If you do:

```go
{
    client := metric.NewClient(sink)
    counter := client.RegisterCounter("name")
    counter.Inc(1)
    counter.Inc(1)
}
```

Then buffering will only send an increase of "2" to the sink - and only put it on the wire when the sink flushes.

On the other hand, if you do:

```go
{
    client.AdhocCount("name",1, false)
    client.AdhocCount("name",1, true)
}
```

Then both metric events will be sent to the sink and the latter will also ask the sink to flush data to the wire immediately.

## Example

```go
package main

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"log"
	"time"
)

func main() {

	flushPeriod  := metric.FlushInterval(4*time.Second)
	timerFlushPeriod := metric.FlushInterval(2*time.Second)

	var sink metric.Sink
	var err error

	sink, err = statsd.New(
		statsd.Buffer(512),
		//statsd.Peer("statsdhost:8125"), // uncomment to send UDP data.
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

	client := metric.NewClient(sink, flushPeriod)

	gauge   := client.RegisterGauge("gauge")
	timer   := client.RegisterTimer("timer", timerFlushPeriod)
	histo   := client.RegisterHistogram("histo")
	counter := client.RegisterCounter("counter")  // all using default client flushperiod

	client.Start()

	var g int
	for g < 100 {
		counter.Inc(1)
		gauge.Set(uint64(g))
		timer.Sample(time.Duration(g)*time.Millisecond)
		histo.Sample(int64(g))

		time.Sleep(time.Second)
		g++
	}
	client.Stop()
	client.Flush()
}
```
