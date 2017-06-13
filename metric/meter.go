package metric

// Conceptual meter types
// Gauge: A client side maintained counter
// Counter: A server side maintained counter
// Historam: A series of events analyzed on the server
// Timer: A series of time.Duration events analyzed on the server
// Set: Discrete strings added to a set maintained on the server
const (
	MeterGauge     = iota // A client side maintained value
	MeterCounter          // A server side maintained value
	MeterHistogram        // A general distribution of measurements events.
	MeterTimer            // ... when those measurements are milliseconds
	MeterSet              // free form string events
)

// Meter is a named metrics measurement as seen from the Client.
// It measures stuff and can be registered with a client to
// be periodically reported to the Sink.
// This interface doesn't describe how measurements are done. That depends
// on the specific meter it self. This interface only makes the Client able
// to flush the meter to a sink.
type meter interface {
	//	MeterType() int
	Name() string
	Flush(Sink) // Read the meter, by flushing all non-read values.
}

// An AutoFlusher can initiate a Flush through the flusher at any time and needs
// to know the Flusher to call FlushMeter() on it
type autoFlusher interface {
	SetFlusher(*flusher)
}
