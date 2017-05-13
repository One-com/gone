package reaper

import (
	"sync/atomic"
	"time"
)

type element struct {
	next *element
	conn *conn
}

func reaper(first *conn, incomming <-chan *conn, interval time.Duration, maxMiss int64, exitCounter *uint32) {

	ticker := time.Tick(interval)
	var head *conn = first
	var curr, prev *conn
	var emptyCycleRun int
	var removeConnection bool
	var active uint64

	for emptyCycleRun < 2 {
		select {
		case newConn := <-incomming:
			emptyCycleRun = 0
			newConn.next = head
			head = newConn
		case <-ticker:
			if head == nil {
				emptyCycleRun++
			}
			curr = head // pick the first element
			prev = nil
			for curr != nil {
				// assume connection is still alive
				removeConnection = false

				active = atomic.LoadUint64(&curr.activeCount)

				//fmt.Println(active)
				// check closed status flag first
				if active&1 != 0 {
					removeConnection = true
				} else if curr.ioActivityTimeoutEnabled.isSet() {
					if active == curr.lastActiveCount {
						// nothing has changed since last reaper cycle.
						curr.reaperMiss++
						if curr.reaperMiss >= maxMiss {
							removeConnection = true
						}
					} else {
						// connection i alive
						curr.lastActiveCount = active
						curr.reaperMiss = 0
					}
				}

				// Try removing the connection from registry, we tend to remove the reference
				// only if there are no traces of the connection being used for Read/Write and Close calls
				if removeConnection && curr.tryClose() {
					// if the first element of list
					if prev == nil {
						head = head.next
					} else {
						prev.next = curr.next // unchain the element.
					}
				} else {
					prev = curr
				}

				curr = curr.next
			}
		}
	}
	// Decrement the reaper counter
	atomic.AddUint32(exitCounter, ^uint32(0))
}
