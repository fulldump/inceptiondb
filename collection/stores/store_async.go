package stores

import (
	"errors"
	"sync"
	"sync/atomic"
)

type asyncReq struct {
	op   uint8
	id   int64
	data []byte
	sync bool
	done chan error
}

const numStoreShards = 16

type storeAsyncShard struct {
	_    [64]byte // padding to prevent false sharing between shards
	mu   sync.Mutex
	reqs []asyncReq
	_    [64]byte // padding
}

// StoreAsync is a wrapper that implements a sharded buffered queue and Group Commit pattern.
// Each producer goroutine appends to its own shard (round-robin), eliminating mutex contention.
// The worker sweeps all shards, drains them, and writes to disk in batches.
type StoreAsync struct {
	store  Store
	shards [numStoreShards]storeAsyncShard
	wakeup chan struct{} // buffered(1) coalescing signal
	closed atomic.Bool
	done   chan struct{}
	picker atomic.Uint32
}

func NewStoreAsync(store Store) *StoreAsync {
	s := &StoreAsync{
		store:  store,
		wakeup: make(chan struct{}, 1),
		done:   make(chan struct{}),
	}
	for i := range s.shards {
		s.shards[i].reqs = make([]asyncReq, 0, 8192)
	}
	go s.worker()
	return s
}

// sweep collects all pending requests from all shards into batch.
func (s *StoreAsync) sweep(batch []asyncReq) []asyncReq {
	for i := range s.shards {
		sh := &s.shards[i]
		sh.mu.Lock()
		batch = append(batch, sh.reqs...)
		sh.reqs = sh.reqs[:0]
		sh.mu.Unlock()
	}
	return batch
}

func (s *StoreAsync) worker() {
	var batch []asyncReq

	for {
		// Sweep all shards for pending work
		batch = s.sweep(batch)

		if len(batch) > 0 {
			// Process the batch
			needsSync := false
			var err error
			for _, r := range batch {
				if appendErr := s.store.Append(r.op, r.id, r.data, false); appendErr != nil && err == nil {
					err = appendErr
				}
				if r.sync {
					needsSync = true
				}
			}

			if err == nil {
				if needsSync {
					err = s.store.Sync()
				} else {
					err = s.store.Flush()
				}
			}

			for _, r := range batch {
				if r.done != nil {
					r.done <- err
				}
			}

			batch = batch[:0]
			// Loop immediately to check for more data without blocking
			continue
		}

		// No data found. Check if we should exit.
		if s.closed.Load() {
			break
		}

		// Block until a producer signals or channel closes
		_, ok := <-s.wakeup
		if !ok {
			break
		}
	}

	// Final drain: sweep any remaining data added after last check
	batch = s.sweep(batch)
	if len(batch) > 0 {
		var err error
		for _, r := range batch {
			if appendErr := s.store.Append(r.op, r.id, r.data, false); appendErr != nil && err == nil {
				err = appendErr
			}
		}
		if err == nil {
			err = s.store.Flush()
		}
		for _, r := range batch {
			if r.done != nil {
				r.done <- err
			}
		}
	}

	close(s.done)
}

// Append pushes the operation to a shard. If sync is true, it blocks until physically persisted.
func (s *StoreAsync) Append(op uint8, id int64, data []byte, fsync bool) error {
	if s.closed.Load() {
		return errors.New("store closed")
	}

	var done chan error
	if fsync {
		done = make(chan error, 1)
	}

	// Pick a shard via round-robin (fast atomic, zero contention on data path)
	idx := s.picker.Add(1) & (numStoreShards - 1)
	sh := &s.shards[idx]
	sh.mu.Lock()
	sh.reqs = append(sh.reqs, asyncReq{
		op:   op,
		id:   id,
		data: data,
		sync: fsync,
		done: done,
	})
	sh.mu.Unlock()

	// Non-blocking coalescing signal to worker.
	// If the channel already has a signal, this is a no-op (the worker will sweep all shards).
	select {
	case s.wakeup <- struct{}{}:
	default:
	}

	if fsync {
		return <-done
	}
	return nil
}

func (s *StoreAsync) Flush() error { return s.store.Flush() }

func (s *StoreAsync) Sync() error { return s.store.Sync() }

// Close triggers a shutdown of the worker and waits for it to drain before closing the underlying store.
func (s *StoreAsync) Close() error {
	s.closed.Store(true)
	close(s.wakeup)
	<-s.done
	return s.store.Close()
}

func (s *StoreAsync) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return s.store.Replay(fn)
}
