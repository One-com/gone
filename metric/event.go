package metric

import (
	"runtime"
	"sync"
	"sync/atomic"
)

// An almost lock-free FIFO buffer
// Locks are only used when flushing

const bufferMaskBits = 8 // determines the size of the buffer
const bufferSize = uint64(1) << bufferMaskBits
const bufferMask = bufferSize - 1

const indexStart = 0

type event struct {
	seq uint64
	val uint64 // union of 64-bit numeric values including float64
	mu  sync.Mutex
	cv  *sync.Cond
}

type dequeueFunc func(f Sink, val uint64)

// A generic stream of values which all have to be propagated to the sink.
type eventStream struct {
	widx  uint64 // index of next free slot
	ridx  uint64 // index of next unread slot
	slots [bufferSize]event

	flusher *flusher

	dequeue dequeueFunc

	name string
}

func newEventStream(name string, dqf dequeueFunc) *eventStream {
	e := &eventStream{name: name, dequeue: dqf, widx: indexStart, ridx: indexStart}

	// make sure first slot is not valid from the start due to zero-value
	// and set all sequences to their "old" value
	for i := range e.slots {
		e.slots[i].seq = uint64(i) - bufferSize
		e.slots[i].cv = sync.NewCond(&(e.slots[i].mu))
	}
	return e
}

func (e *eventStream) setFlusher(f *flusher) {
	e.flusher = f
}

// FlushReading - flush as much as possible.
func (e *eventStream) FlushReading(s Sink) {

	var idx uint64

	// Precondition: e.ridx points to next un-eaten slot
	ridx := atomic.LoadUint64(&e.ridx)
	for {
		idx = ridx & bufferMask

		// test if next un-eaten slot has new data
		mark := atomic.LoadUint64(&(e.slots[idx].seq))
		// either tagged with its slot, or +1 for waited on
		if mark == ridx || mark == ridx+1 {
			e.dequeue(s, e.slots[idx].val)
			ridx++
		} else {
			// we've reached a not yet written slot
			break
		}
	}
	// how far did we get?
	atomic.StoreUint64(&(e.ridx), ridx)
}

func (e *eventStream) Name() string {
	return e.name
}

func (e *eventStream) enqueue(val uint64) {

	var ridx uint64
	var widx uint64
	var idx uint64

	// First get a slot
	widx = atomic.AddUint64(&(e.widx), 1)
	widx-- // back up to get our reserved slot
	idx = widx & bufferMask

	var try int
	// we now have widx holding the index we intend to write
	// Then write the data
	for {
		try++
		// Where's the reader? Don't overtake it.
		ridx = atomic.LoadUint64(&e.ridx)

		diff := widx - ridx // unsigned artimetic should work
		if diff < bufferSize {
			// We have not catched up
			e.slots[idx].val = val
			// mark the slot written
			oldmark := atomic.SwapUint64(&(e.slots[idx].seq), widx)

			// test to see if someone was waiting for that mark
			if oldmark != widx-bufferSize {
				// ensure we don't signal before the waiter waits
				e.slots[idx].mu.Lock()
				e.flusher.FlushMeter(e)
				e.slots[idx].cv.Broadcast() // wake up time
				e.slots[idx].mu.Unlock()
			}
			break
		}

		if try == 1 {
			// try again
			runtime.Gosched()
			continue
		}
		// at the time we read ridx, we could not proceed. That may have changed however, so
		// we need to make an atomic operation which:
		// 1) Decides whether to go to sleep and wait for our slot to be ready.
		// 2) Informs the writer of the slot that we want to be woken.

		// The slot we are waiting for have sequence 1 buffersize back from rdix,
		// if it's still not ready

		oldmark := ridx - bufferSize

		idx2 := ridx & bufferMask // from here on we look at the stale read index.
		e.slots[idx2].mu.Lock()
		// Try skew the mark to indicate we're waiting
		mustwait := atomic.CompareAndSwapUint64(&(e.slots[idx2].seq), oldmark, oldmark+1)
		if mustwait {
			// We have now at the same time determined that the slot is not ready
			// and set it to indicate that who ever writes it must signal us.

			e.slots[idx2].cv.Wait()

		} else {
			// Ok... so the slot is not just "old". It has either been updated
			// to current, or someone else has skewed the mark and a signal will
			// be sent to waiters. Find out whether to join the waiters or just try again.
			actualmark := atomic.LoadUint64(&(e.slots[idx2].seq))
			if actualmark == oldmark+1 {
				// skewed - join the waiters.
				e.slots[idx2].cv.Wait()
			} else { // stuff can happen fast
				// The slot was actually up to date - so advance the reader
				e.flusher.FlushMeter(e)
			}
		}
		e.slots[idx2].mu.Unlock()

	}
}
