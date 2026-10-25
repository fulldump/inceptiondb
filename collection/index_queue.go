package collection

import "sync"

// indexQueueMax bounds the pending async index operations: writers block
// when the index worker falls this far behind.
const indexQueueMax = 1000000

// indexQueueKeepCap is the largest buffer the worker keeps after a drain.
// Bigger buffers (a burst of writes) are released so idle collections stay small.
const indexQueueKeepCap = 4096

// indexQueue is the queue of async index operations of a collection. Unlike a
// buffered channel, its memory grows with the backlog instead of being
// allocated upfront, which matters with thousands of mostly idle collections.
type indexQueue struct {
	mu      sync.Mutex
	notFull *sync.Cond
	reqs    []asyncIndexReq
	closed  bool
	wakeup  chan struct{} // buffered(1) coalescing signal
	done    chan struct{} // closed when the worker has exited
}

func newIndexQueue() *indexQueue {
	q := &indexQueue{
		wakeup: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	q.notFull = sync.NewCond(&q.mu)
	return q
}

// push enqueues r, blocking while the queue is full. Requests pushed after
// close are dropped: callers stop writing before closing the queue.
func (q *indexQueue) push(r asyncIndexReq) {
	q.mu.Lock()
	for len(q.reqs) >= indexQueueMax && !q.closed {
		q.notFull.Wait()
	}
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.reqs = append(q.reqs, r)
	q.mu.Unlock()

	select {
	case q.wakeup <- struct{}{}:
	default:
	}
}

// close makes the worker exit once every pending request is processed.
func (q *indexQueue) close() {
	q.mu.Lock()
	q.closed = true
	q.notFull.Broadcast()
	q.mu.Unlock()

	select {
	case q.wakeup <- struct{}{}:
	default:
	}
	<-q.done
}

// run calls process with every batch of pending requests, in push order,
// until the queue is closed and drained.
func (q *indexQueue) run(process func(batch []asyncIndexReq)) {
	defer close(q.done)

	var batch []asyncIndexReq
	for {
		q.mu.Lock()
		batch, q.reqs = q.reqs, batch[:0]
		closed := q.closed
		q.notFull.Broadcast()
		q.mu.Unlock()

		if len(batch) == 0 {
			if closed {
				return
			}
			<-q.wakeup
			continue
		}

		process(batch)
		clear(batch) // drop payload references for the GC
		if cap(batch) > indexQueueKeepCap {
			batch = nil
		}
	}
}
