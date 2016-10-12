package main

import (
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"fmt"

	"log"
	"time"
//	"strconv"
	"io"
	"os"
	"sync"
)

type blockOut struct {
	mu sync.Mutex
	out io.Writer
}

func (b *blockOut) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	n, err = b.out.Write(p)
	b.out.Write([]byte("\n-----------\n"))
	b.mu.Unlock()
	return
}

var period = metric.FlushInterval(2*time.Second)

func main() {

	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Output(&blockOut{out:os.Stdout}),
		statsd.Prefix("prefix"))
	if err != nil {
		log.Fatal(err)
	}

//	client.Gauge("gauge", 42, false)
//	client.Counter("c", 1, true)

	
	client := metric.NewClient(sink, period)

	gauge   := client.NewGauge("gauge",period)
	timer   := client.NewTimer("timer", period)
	histo   := client.NewHistogram("histo",period)
	counter := client.NewCounter("counter",period)
//	set     := client.NewSet("set", period)

	g := 100
	for g != 0 {
		client.Gauge(fmt.Sprintf("g%d",g), uint64(g), false)
		client.Counter(fmt.Sprintf("c%d",g), 1, false)
		counter.Inc(1)
		gauge.Set(uint64(g))
		timer.Sample(time.Duration(g)*time.Millisecond)
		histo.Sample(int64(g))
//		set.Update(strconv.FormatInt(int64(g), 10))
		
		time.Sleep(time.Second)
		g--
		if g % 2 == 0 {
			if g % 4 == 0 {
				client.SetOptions(metric.FlushInterval(time.Second))
			} else {
				client.SetOptions(metric.FlushInterval(0))
			}
		}
	}
	client.Stop()

	client.Flush()
}


