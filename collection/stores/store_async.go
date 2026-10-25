package stores

import (
	"errors"
	"sync"
	"sync/atomic"
)

type asyncReq struct {
	op      uint8
	id      int64
	data    []byte
	sync    bool
	barrier bool // no payload: only waits for everything enqueued before it
	done    chan error
}

const numStoreShards = 16

var ErrStoreClosed = errors.New("store closed")

type storeAsyncShard struct {
	_    [64]byte // padding to prevent false sharing between shards
	mu   sync.Mutex
	reqs []asyncReq
	_    [64]byte // padding
}

// StoreAsync is a wrapper that implements a sharded buffered queue and Group Commit pattern.
// Producers append to the shard of the record id, so operations on the same id
// keep their order in the WAL. The worker sweeps all shards, drains them, and
// writes to disk in batches.
//
// The first write or sync error is sticky: every later operation fails with
// it, because after a failed fsync the state of the file is unknown.
type StoreAsync struct {
	store  Store
	shards [numStoreShards]storeAsyncShard
	wakeup chan struct{} // buffered(1) coalescing signal
	stop   chan struct{}
	closed atomic.Bool
	failed atomic.Pointer[error]
	done   chan struct{}
}

func NewStoreAsync(store Store) *StoreAsync {
	s := &StoreAsync{
		store:  store,
		wakeup: make(chan struct{}, 1),
		stop:   make(chan struct{}),
		done:   make(chan struct{}),
	}
	// Shard queues start empty and keep the capacity they grow to (sweep
	// reuses them), so idle collections do not hold 16 preallocated queues.
	go s.worker()
	return s
}

func (s *StoreAsync) err() error {
	if err := s.failed.Load(); err != nil {
		return *err
	}
	return nil
}

func (s *StoreAsync) fail(err error) {
	s.failed.CompareAndSwap(nil, &err)
}

// sweep collects all pending requests from all shards into batch. Shards are
// read in order, so a barrier enqueued in shard 0 is only collected together
// with (or after) every request enqueued before it in any shard.
func (s *StoreAsync) sweep(batch []asyncReq) []asyncReq {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		batch = append(batch, sh.reqs...)
		clear(sh.reqs) // drop payload references for the GC
		sh.reqs = sh.reqs[:0]
		sh.mu.Unlock()
	}
	return batch
}

func (s *StoreAsync) process(batch []asyncReq) {
	err := s.err()
	needsSync := false
	if err == nil {
		for _, r := range batch {
			if r.sync {
				needsSync = true
			}
			if r.barrier {
				continue
			}
			if appendErr := s.store.Append(r.op, r.id, r.data, false); appendErr != nil {
				err = appendErr
				break
			}
		}
	}

	if err == nil {
		if needsSync {
			err = s.store.Sync()
		} else {
			err = s.store.Flush()
		}
	}
	if err != nil {
		s.fail(err)
		err = s.err()
	}

	for _, r := range batch {
		if r.done != nil {
			r.done <- err
		}
	}
}

func (s *StoreAsync) worker() {
	defer close(s.done)

	var batch []asyncReq
	for {
		batch = s.sweep(batch)
		if len(batch) > 0 {
			s.process(batch)
			clear(batch)
			batch = batch[:0]
			// Loop immediately to check for more data without blocking
			continue
		}

		select {
		case <-s.wakeup:
		case <-s.stop:
			// Close has fenced every shard: this sweep sees the last requests
			batch = s.sweep(batch)
			if len(batch) > 0 {
				s.process(batch)
			}
			return
		}
	}
}

func (s *StoreAsync) enqueue(shard uint32, r asyncReq) error {
	sh := &s.shards[shard&(numStoreShards-1)]
	sh.mu.Lock()
	// Checked under the shard lock: Close fences every shard after setting
	// closed, so a request is either rejected here or drained by the worker.
	if s.closed.Load() {
		sh.mu.Unlock()
		return ErrStoreClosed
	}
	sh.reqs = append(sh.reqs, r)
	sh.mu.Unlock()

	// Non-blocking coalescing signal to worker.
	// If the channel already has a signal, this is a no-op (the worker will sweep all shards).
	select {
	case s.wakeup <- struct{}{}:
	default:
	}
	return nil
}

// Append pushes the operation to the shard of id. If fsync is true, it blocks
// until physically persisted.
func (s *StoreAsync) Append(op uint8, id int64, data []byte, fsync bool) error {
	if err := s.err(); err != nil {
		return err
	}

	var done chan error
	if fsync {
		done = make(chan error, 1)
	}

	err := s.enqueue(uint32(id), asyncReq{
		op:   op,
		id:   id,
		data: data,
		sync: fsync,
		done: done,
	})
	if err != nil {
		return err
	}

	if fsync {
		return <-done
	}
	return nil
}

// barrier waits until every request enqueued before it has been written, and
// fsynced when fsync is true.
func (s *StoreAsync) barrier(fsync bool) error {
	if err := s.err(); err != nil {
		return err
	}
	done := make(chan error, 1)
	// Shard 0 is the first one swept, see sweep
	if err := s.enqueue(0, asyncReq{barrier: true, sync: fsync, done: done}); err != nil {
		return err
	}
	return <-done
}

func (s *StoreAsync) Flush() error { return s.barrier(false) }

func (s *StoreAsync) Sync() error { return s.barrier(true) }

// Close triggers a shutdown of the worker and waits for it to drain before closing the underlying store.
func (s *StoreAsync) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	// Fence: after this, no Append can enqueue (see enqueue)
	for i := range s.shards {
		s.shards[i].mu.Lock()
		s.shards[i].mu.Unlock()
	}
	close(s.stop)
	<-s.done

	closeErr := s.store.Close()
	if err := s.err(); err != nil {
		return err
	}
	return closeErr
}

func (s *StoreAsync) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return s.store.Replay(fn)
}
