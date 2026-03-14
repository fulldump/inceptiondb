package collectionv4

import "errors"

type asyncReq struct {
	op   uint8
	id   int64
	data []byte
	sync bool
	done chan error
}

// StoreAsync is a wrapper that implements a buffered queue and Group Commit pattern.
// It drains pending requests as fast as possible, buffers them to the OS, and syncs
// the physical disk only when a request explicitly demands it (WriteConcern).
type StoreAsync struct {
	store Store
	reqs  chan asyncReq
	done  chan struct{}
}

func NewStoreAsync(store Store) *StoreAsync {
	s := &StoreAsync{
		store: store,
		reqs:  make(chan asyncReq, 1000000), // Huge buffer for high throughput 
		done:  make(chan struct{}),
	}
	go s.worker()
	return s
}

func (s *StoreAsync) worker() {
	var batch []asyncReq
	for req := range s.reqs {
		batch = append(batch, req)

		// Drain as many pending requests as possible to group them
	drain:
		for len(batch) < 10000 {
			select {
			case r, ok := <-s.reqs:
				if !ok {
					break drain
				}
				batch = append(batch, r)
			default:
				break drain
			}
		}

		needsSync := false
		for _, r := range batch {
			// We append setting sync=false internally, delegating the physical sync to the batch end
			_ = s.store.Append(r.op, r.id, r.data, false)
			if r.sync {
				needsSync = true
			}
		}

		var err error
		if needsSync {
			// If at least one request wanted physical persistence, sync the whole batch to disk
			err = s.store.Sync()
		} else {
			// Otherwise just flush the user-space buffer to the OS Page Cache
			err = s.store.Flush()
		}

		// Broadcast the result back to those waiting
		for _, r := range batch {
			if r.done != nil {
				r.done <- err
			}
		}
		
		// Reset batch allocation
		batch = batch[:0]
	}

	close(s.done)
}

// Append pushes the operation to the queue. If sync is true, it blocks until it is physically persisted.
func (s *StoreAsync) Append(op uint8, id int64, data []byte, sync bool) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = errors.New("store closed")
		}
	}()

	var done chan error
	if sync {
		done = make(chan error, 1) // Buffered to avoid blocking the worker
	}

	s.reqs <- asyncReq{
		op:   op,
		id:   id,
		data: data,
		sync: sync,
		done: done,
	}

	if sync {
		return <-done
	}
	return nil
}

func (s *StoreAsync) Flush() error { return s.store.Flush() }

func (s *StoreAsync) Sync() error  { return s.store.Sync() }

// Close triggers a shutdown of the worker queue and waits for it to drain before closing the underlying store.
func (s *StoreAsync) Close() error {
	close(s.reqs)
	<-s.done
	return s.store.Close()
}

func (s *StoreAsync) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return s.store.Replay(fn)
}
