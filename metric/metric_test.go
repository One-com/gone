package metric_test

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"log"
	"time"
	//	"testing"
	"os"
)

func ExampleNewClient() {
	
	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Output(os.Stdout),
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

	c := metric.NewClient(sink)

	gauge1 := c.RegisterGauge("gauge1")
	gauge2 := metric.NewGauge("gauge2")
	c.Register(gauge2)
	counter := c.RegisterCounter("counter")
	timer := c.RegisterTimer("timer")
	histo := c.RegisterHistogram("histo")

	gauge1.Set(17)
	gauge2.Set(18)
	counter.Inc(1)
	timer.Sample(time.Duration(10 * time.Millisecond))
	histo.Sample(17)

	c.Flush()
	// Output:
	// prefix.gauge1:17|g
	// prefix.gauge2:18|g
	// prefix.counter:1|c
	// prefix.timer:10|ms
	// prefix.histo:17|ms

}
