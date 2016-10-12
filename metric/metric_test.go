package metric_test

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"log"
	"time"
)

func ExampleNewClient() {

	var _flushPeriod = 4*time.Second

	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Peer("127.0.0.2:8125"),
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

	flushPeriod := metric.FlushInterval(_flushPeriod)

	c := metric.NewClient(sink,flushPeriod)
	
	gauge   := c.NewGauge("gauge",flushPeriod)
	timer   := c.NewTimer("timer")
	histo   := c.NewHistogram("histo",flushPeriod)
	counter := c.NewCounter("counter",flushPeriod)

	g := 100
	for g != 0 {
		counter.Inc(1)
		gauge.Set(uint64(g))
		timer.Sample(time.Duration(g)*time.Millisecond)
		histo.Sample(int64(g))
		
		time.Sleep(time.Second)
		g--
	}
	c.Stop()
	
}

