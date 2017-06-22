package metric_test

import (
	"bytes"
	"github.com/One-com/gone/metric"
	"github.com/One-com/gone/metric/sink/statsd"
	"sort"
	"strings"
	"testing"
)

func TestSet(t *testing.T) {

	var buffer = &bytes.Buffer{}

	sink, err := statsd.New(
		statsd.Buffer(512),
		statsd.Output(buffer),
		statsd.Prefix("pfx"))
	if err != nil {
		t.Fatal(err)
	}

	set := metric.NewSet("set")
	metric.Default().Register(set)

	metric.AdhocSetMember("set", "nosink1", false)
	metric.AdhocGauge("g", 2, false) // just do this to activate the nilsink Numeric64
	set.Add("nosink2")
	metric.Flush()

	output := buffer.Bytes()
	if string(output) != "" {
		t.Errorf("Wrong output %s", output)
	}

	metric.SetDefaultSink(sink)

	metric.AdhocSetMember("set", "member1", false)
	set.Add("member1")
	set.Add("member2")
	set.Add("member2")
	set.Add("member3")

	metric.Flush()

	outbytes := buffer.Bytes()
	outstring := string(outbytes)
	lines := strings.Split(outstring, "\n")
	if len(lines) != 5 { // ending in empty string after last \n
		t.Errorf("Unexpected number of set members %d\n %s", len(lines), strings.Join(lines, "<NL>"))
	}
	// Remove last empty element.
	if lines[len(lines)-1] == "" {
		lines = lines[0 : len(lines)-1]
	} else {
		t.Error("Output should have ended in newline")
	}
	sort.Strings(lines)
	result := strings.Join(lines, "\n")
	if result != "pfx.set:member1|s\npfx.set:member1|s\npfx.set:member2|s\npfx.set:member3|s" {
		t.Errorf("Wrong output %s", outstring)
	}

	// Prevent this from mixing into other tests
	metric.Default().Deregister(set)

}
