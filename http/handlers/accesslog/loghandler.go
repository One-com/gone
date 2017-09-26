package accesslog

import (
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/One-com/gone/http/rrwriter"

	"unsafe"
)

var lastReportedAccesslogError atomic.Value

func init() {
	lastReportedAccesslogError.Store(time.Unix(0, 0))
}

type buffer [256]byte // temporary byte array for creating log lines.

// DynamicLogHandler is the interface of a logging http.Handler capable
// of turning on/off access log writing to multiple io.Writer
type DynamicLogHandler interface {
	http.Handler
	// ToggleAccessLog controls which io.Writer accesslog is written to.
	// call (nil, w) to turn accesslog on a io.Writer on.
	// call (w, nil) to turn the accesslog of an io.Writer off
	// call (nil,nil) to turn accesslog completely off.
	// call (old,new) to atomically which accesslog from one io.Writer to another
	ToggleAccessLog(old, new io.Writer)
}

// AuditFunction is a function which will be called by a DynamicLogHandler after a
// HTTP request has been processed. It can be used for metrics by inspecting the state
// of the RecordingResponseWriter.
type AuditFunction func(rrwriter.RecordingResponseWriter)

type logHandler struct {
	handler http.Handler
	bufpool *sync.Pool
	mu      sync.Mutex
	writers []io.Writer
	out     unsafe.Pointer // pointer to io.Writer
	af      AuditFunction
}

// NewDynamicLogHandler wraps around a provided handler and returns a DynamicLogHandler
// capable of turning accesslog on/off dynamically.
// If provided an AuditFunction it will be called after ServeHTTP on wrapped handler
// returns
func NewDynamicLogHandler(h http.Handler, af AuditFunction) DynamicLogHandler {
	p := &sync.Pool{New: func() interface{} { return new(buffer) }}
	return &logHandler{handler: h, bufpool: p, af: af}
}

func (h *logHandler) ToggleAccessLog(old, new io.Writer) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if old == nil { // Enable logging for new io.Writer
		h.writers = append(h.writers, new)

		out := io.MultiWriter(h.writers...)

		//h.out.Store(out)
		atomic.StorePointer(&h.out, unsafe.Pointer(&out))
		return
	}

	// walk through the writers, find the one needing to be retired.
	var i, j int
	for i < len(h.writers) {
		wo := h.writers[i]
		var wn io.Writer
		if wo == old {
			if new == nil { // remove the log writer
				i++
				continue
			}
			wn = new // put in the new

		} else {
			// keep the old
			wn = h.writers[i]
		}
		h.writers[j] = wn
		j++
		i++
	}
	h.writers = h.writers[:j]

	if len(h.writers) == 0 {
		atomic.StorePointer(&h.out, unsafe.Pointer(nil))
		//h.out.Store(nil)
	} else {
		out := io.MultiWriter(h.writers...)
		atomic.StorePointer(&h.out, unsafe.Pointer(&out))
		//h.out.Store(out)
	}
}

func (h *logHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	//out := h.out.Load()

	var out unsafe.Pointer
	out = atomic.LoadPointer(&h.out)

	if out != nil {

		outw := *((*io.Writer)(out))

		recorder := rrwriter.MakeRecorder(w)

		t := time.Now()
		recorder.SetTimeStamp(t)

		pbuf := h.bufpool.Get().(*buffer)
		logbuf := pbuf[:0]

		logbuf = preBuildLogLine(logbuf, req, t)
		h.handler.ServeHTTP(recorder, req)
		logbuf = postBuildLogLine(logbuf, recorder)

		err := writeLog(outw, logbuf)
		if err != nil {
			last := lastReportedAccesslogError.Load().(time.Time)
			if t.Sub(last) > time.Minute {
				server := req.Context().Value(http.ServerContextKey)
				if s, ok := server.(*http.Server); ok {
					if s.ErrorLog != nil {
						s.ErrorLog.Print(fmt.Sprintf("Error writing access log, suppressing for a minute: %s", err))
						lastReportedAccesslogError.Store(t)
					}
				}
			}
		}
		h.bufpool.Put(pbuf)

		if h.af != nil {
			h.af(recorder)
		}

	} else {
		if h.af != nil {
			recorder := rrwriter.MakeRecorder(w)
			//recorder.SetTimeStamp(time.Now())
			h.handler.ServeHTTP(recorder, req)
			h.af(recorder)
		} else {
			h.handler.ServeHTTP(w, req)
		}
	}
}
