package metric_test

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"log"
	"time"
//	"testing"
	"os"
)

//func ExampleNewClient() {
//
//	var _flushPeriod = 4 * time.Second
//
//	var sink metric.SinkFactory
//	var err error
//
//	sink, err = statsd.New(
//		statsd.Buffer(512),
//		statsd.Peer("127.0.0.2:8125"),
//		statsd.Prefix("prefix"))
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	flushPeriod := metric.FlushInterval(_flushPeriod)
//
//	c := metric.NewClient(sink, flushPeriod)
//
//	gauge := c.NewGauge("gauge", flushPeriod)
//	timer := c.NewTimer("timer")
//	histo := c.NewHistogram("histo", flushPeriod)
//	counter := c.NewCounter("counter", flushPeriod)
//
//	g := 100
//	for g != 0 {
//		counter.Inc(1)
//		gauge.Set(uint64(g))
//		timer.Sample(time.Duration(g) * time.Millisecond)
//		histo.Sample(int64(g))
//
//		time.Sleep(time.Second)
//		g--
//	}
//	c.Stop()
//
//}

func ExampleNewClient() {
	var sink metric.SinkFactory
	var err error

	sink, err = statsd.New(
		statsd.Buffer(512),
		statsd.Output(os.Stdout),
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

	c := metric.NewClient(sink)

	gauge := c.NewGauge("gauge")
	counter := c.NewCounter("counter")
	timer := c.NewTimer("timer")
	histo := c.NewHistogram("histo")

	gauge.Set(23)
	counter.Inc(1)
	timer.Sample(time.Duration(10*time.Millisecond))
	histo.Sample(17)
	
	c.Flush()
	// Output:
	// prefix.gauge:23|g
	// prefix.counter:1|c
	// prefix.timer:10|ms
	// prefix.histo:17|ms

}
