package collectionv4

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
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
}

func NewStoreCrazy(path string) (*StoreCrazy, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
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
func (s *StoreCrazy) Append(op uint8, id int64, data []byte) error {
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
	_, err := s.writer.Write(buf)
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
	return s.writer.Flush()
}

func (s *StoreCrazy) Sync() error {
	if s.closed.Load() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.writer.Flush(); err != nil {
		return err
	}
	return s.file.Sync()
}

func (s *StoreCrazy) Close() error {
	if s.closed.Swap(true) {
		return nil
	}
	
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.file.Sync(); err != nil {
		return err
	}
	return s.file.Close()
}

func (s *StoreCrazy) Replay(fn func(op uint8, id int64, data []byte) error) error {
	f, err := os.Open(s.file.Name())
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 4*1024*1024)
	header := make([]byte, 13)

	for {
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("WAL crazy header abruptly cut")
			}
			return err
		}

		op := header[0]
		id := int64(binary.LittleEndian.Uint64(header[1:9]))
		length := binary.LittleEndian.Uint32(header[9:13])

		var data []byte
		if length > 0 {
			data = make([]byte, length)
			_, err = io.ReadFull(reader, data)
			if err != nil {
				return fmt.Errorf("WAL crazy payload cut at ID %d: %v", id, err)
			}
			// No CRC verification step.
		}

		if err := fn(op, id, data); err != nil {
			return err
		}
	}

	return nil
}
