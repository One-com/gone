package metric_test

import (
	"bytes"
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"testing"
	"time"
)

func TestReplaceNilSink(t *testing.T) {

	var buffer = &bytes.Buffer{}

	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Output(buffer),
		statsd.Prefix("pfx"))
	if err != nil {
		t.Fatal(err)
	}

	gauge := metric.Default().RegisterGauge("gauge")

	gauge.Set(23)
	metric.Flush()

	output := buffer.Bytes()
	if string(output) != "" {
		t.Errorf("Wrong output %s", output)
	}

	metric.SetDefaultSink(sink)

	gauge.Set(24)
	metric.Flush()

	output = buffer.Bytes()
	if string(output) != "pfx.gauge:24|g\n" {
		t.Errorf("Wrong output %s", output)
	}

	// Prevent this from mixing into other tests
	metric.Default().Deregister(gauge)

}

func TestFlushSink(t *testing.T) {

	var buffer = &bytes.Buffer{}

	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Output(buffer),
		statsd.Prefix("pfx"))
	if err != nil {
		t.Fatal(err)
	}

	metric.SetDefaultSink(sink)
	metric.SetDefaultOptions(metric.FlushInterval(500 * time.Millisecond))

	gauge := metric.Default().RegisterGauge("auto")

	gauge.Set(25)

	// It should not have been flushed yet.
	output := buffer.Bytes()
	if string(output) != "" {
		t.Errorf("Wrong output %s", output)
	}

	// Wait for the gauge reading to happen.
	time.Sleep(600 * time.Millisecond)

	// Add another manually and tell it to flush the sink
	metric.Default().AdhocGauge("manual", 26, true)

	output = buffer.Bytes()
	if string(output) != "pfx.auto:25|g\npfx.manual:26|g\n" {
		t.Errorf("Wrong output %s", output)
	}

}
