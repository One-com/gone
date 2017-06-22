package statsd_test

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"log"
	"os"
	"time"
)

func ExampleNew() {
	var sink metric.Sink
	var err error

	sink, err = statsd.New(
		statsd.Buffer(512),
		statsd.Output(os.Stdout),
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

	gauge := metric.NewGauge("gauge")
	counter := metric.NewCounter("counter")
	histo := metric.NewHistogram("histo")
	timer := metric.NewTimer("timer")
	set := metric.NewSet("set")

	timer.Sample(time.Second)
	timer.Sample(time.Millisecond)
	gauge.Set(17)
	histo.Sample(123456)
	counter.Inc(2)
	counter.Inc(3)
	set.Add("member")
	set.Add("member")

	timer.FlushReading(sink)
	gauge.FlushReading(sink)
	counter.FlushReading(sink)
	histo.FlushReading(sink)
	set.FlushReading(sink)
	sink.Flush()
	// Output:
	// prefix.timer:1000|ms
	// prefix.timer:1|ms
	// prefix.gauge:17|g
	// prefix.counter:5|c
	// prefix.histo:123456|ms
	// prefix.set:member|s
}
