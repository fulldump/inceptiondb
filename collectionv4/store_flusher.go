package collectionv4

import (
	"time"
)

// StoreFlusher is a wrapper that decorates any Store
// adding a background goroutine to periodically flush its buffers.
type StoreFlusher struct {
	Store
	done     chan struct{}
	interval time.Duration
}

// NewStoreFlusher wraps an existing Store ensuring it flushes to disk
// periodically, safeguarding buffered data in low-throughput situations.
func NewStoreFlusher(store Store, interval time.Duration) *StoreFlusher {
	s := &StoreFlusher{
		Store:    store,
		done:     make(chan struct{}),
		interval: interval,
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = store.Flush()
			case <-s.done:
				// When closed, trigger a final flush gracefully
				_ = store.Flush()
				return
			}
		}
	}()

	return s
}

// Close stops the background flusher and closes the underlying store.
func (s *StoreFlusher) Close() error {
	close(s.done)
	return s.Store.Close()
}
