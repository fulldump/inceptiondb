package stores

import (
	"bufio"
	"encoding/binary"
	"errors"
	"os"
	"sync"
	"sync/atomic"
)

var storeCrazyBufferPool = &sync.Pool{
	New: func() interface{} {
		// preallocate 1KB buffer minimum
		b := make([]byte, 0, 1024)
		return &b
	},
}

// StoreCrazy is a high-speed variant of StoreDisk that omits CRC checksums.
// It trades corruption guarantees for pure raw throughput.
type StoreCrazy struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
	closed atomic.Bool
	failed error // sticky: set when a write or fsync fails, guarded by mu
}

func NewStoreCrazy(path string) (*StoreCrazy, error) {
	f, err := openWAL(path)
	if err != nil {
		return nil, err
	}
	s := &StoreCrazy{
		file:   f,
		writer: bufio.NewWriterSize(f, 1024*1024), // 1MB buffer
	}

	return s, nil
}

// Append writes the operation to the WAL.
// Header (13 bytes) = OpCode(1) + ID(8) + Length(4) (NO CRC)
func (s *StoreCrazy) Append(op uint8, id int64, data []byte, sync bool) error {
	if s.closed.Load() {
		return errors.New("StoreCrazy closed")
	}

	var header [13]byte
	header[0] = op
	binary.LittleEndian.PutUint64(header[1:9], uint64(id))

	length := uint32(len(data))
	binary.LittleEndian.PutUint32(header[9:13], length)

	bufPtr := storeCrazyBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, header[:]...)
	buf = append(buf, data...)

	s.mu.Lock()
	err := s.failed
	if err == nil {
		if _, err = s.writer.Write(buf); err != nil {
			s.failed = err
		} else if sync {
			err = s.syncLocked()
		}
	}
	s.mu.Unlock()

	*bufPtr = buf
	storeCrazyBufferPool.Put(bufPtr)

	return err
}

func (s *StoreCrazy) Flush() error {
	if s.closed.Load() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed != nil {
		return s.failed
	}
	if err := s.writer.Flush(); err != nil {
		s.failed = err
		return err
	}
	return nil
}

func (s *StoreCrazy) Sync() error {
	if s.closed.Load() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked()
}

// syncLocked flushes the buffer and fsyncs the file. After a failure the
// store refuses every write: the kernel may have dropped the dirty pages, so
// a later successful fsync would not mean the data is on disk.
func (s *StoreCrazy) syncLocked() error {
	if s.failed != nil {
		return s.failed
	}
	if err := s.writer.Flush(); err != nil {
		s.failed = err
		return err
	}
	if err := s.file.Sync(); err != nil {
		s.failed = err
		return err
	}
	return nil
}

func (s *StoreCrazy) Close() error {
	if s.closed.Swap(true) {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.syncLocked()
	if closeErr := s.file.Close(); err == nil {
		err = closeErr
	}
	return err
}

func (s *StoreCrazy) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return replayWAL(s.file, walFormat{headerSize: 13, crc: false}, fn)
}
