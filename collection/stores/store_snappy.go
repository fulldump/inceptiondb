package stores

import (
	"github.com/golang/snappy"
	"sync"
)

// StoreSnappy is a wrapper that compresses payloads using the fast Snappy algorithm.
// It encodes payload data on Append and decodes it on Replay, leaving headers intact.
type StoreSnappy struct {
	store Store
}

var snappyEncodeBufPool = sync.Pool{
	New: func() interface{} {
		// Allocate an initial capacity for snappy encoding buffers
		b := make([]byte, 0, 4096)
		return &b
	},
}

func NewStoreSnappy(store Store) *StoreSnappy {
	return &StoreSnappy{
		store: store,
	}
}

// Append compresses the supplied data and passes it to the underlying store.
func (s *StoreSnappy) Append(op uint8, id int64, data []byte, wait bool) error {
	// For operations with no payload (e.g. Delete, DropIndex)
	if len(data) == 0 {
		return s.store.Append(op, id, data, wait)
	}

	bufPtr := snappyEncodeBufPool.Get().(*[]byte)
	buf := *bufPtr

	// Ensure our buffer has enough capacity
	maxLen := snappy.MaxEncodedLen(len(data))
	if cap(buf) < maxLen {
		buf = make([]byte, 0, maxLen)
	} else {
		buf = buf[:0]
	}

	compressed := snappy.Encode(buf, data)

	err := s.store.Append(op, id, compressed, wait)

	*bufPtr = buf
	snappyEncodeBufPool.Put(bufPtr)

	return err
}

func (s *StoreSnappy) Flush() error {
	return s.store.Flush()
}

func (s *StoreSnappy) Sync() error {
	return s.store.Sync()
}

func (s *StoreSnappy) Close() error {
	return s.store.Close()
}

// Replay reads the underlying store's raw data, decompressing the payloads before yielding.
func (s *StoreSnappy) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return s.store.Replay(func(op uint8, id int64, compressedData []byte) error {
		if len(compressedData) == 0 {
			return fn(op, id, compressedData)
		}

		// snappy.Decode(nil, ...) allocates a new exactly-sized slice.
		// This is ideal because the Recover function typically retains this byte slice in memory forever.
		decompressed, err := snappy.Decode(nil, compressedData)
		if err != nil {
			return err
		}

		return fn(op, id, decompressed)
	})
}
