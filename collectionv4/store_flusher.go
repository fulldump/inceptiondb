package collectionv4

import (
	"time"
)

// StoreFlusher is a wrapper that periodically calls Flush() and Sync() on the
// underlying store in a background goroutine.
type StoreFlusher struct {
	store Store
	done  chan struct{}
}

func NewStoreFlusher(store Store, interval time.Duration) *StoreFlusher {
	sf := &StoreFlusher{
		store: store,
		done:  make(chan struct{}),
	}
	go sf.worker(interval)
	return sf
}

func (s *StoreFlusher) worker(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			_ = s.store.Flush()
			_ = s.store.Sync()
		case <-s.done:
			return
		}
	}
}

func (s *StoreFlusher) Append(op uint8, id int64, data []byte, sync bool) error {
	return s.store.Append(op, id, data, sync)
}

func (s *StoreFlusher) Flush() error {
	return s.store.Flush()
}

func (s *StoreFlusher) Sync() error {
	return s.store.Sync()
}

func (s *StoreFlusher) Close() error {
	close(s.done)
	// Vaciar cualquier búfer pendiente antes de cerrar
	_ = s.store.Flush()
	_ = s.store.Sync()
	return s.store.Close()
}

func (s *StoreFlusher) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return s.store.Replay(fn)
}
