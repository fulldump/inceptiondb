package storage

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

type JSONStorage struct {
	Filename string
	file     *os.File
	buffer   *bufio.Writer
	mu       sync.Mutex
	closed   atomic.Bool
}

func NewJSONStorage(filename string) (*JSONStorage, error) {
	s := &JSONStorage{
		Filename: filename,
	}

	var err error
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	s.buffer = bufio.NewWriterSize(s.file, 16*1024*1024)

	return s, nil
}

func (s *JSONStorage) Persist(command *Command, id string, payload interface{}) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	// Encode outside the lock (concurrent, parallel)
	buf := encodeCommandToBuffer(command)
	data := buf.Bytes()

	// Single Write call inside the lock (minimal critical section)
	s.mu.Lock()
	_, _ = s.buffer.Write(data)
	s.mu.Unlock()

	bufferPool.Put(buf)
	return nil
}

// PersistInsert is a fast-path for insert commands that avoids intermediate heap allocations.
func (s *JSONStorage) PersistInsert(seq uint64, timestamp int64, payload []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	// Build the entire line outside the lock using a pooled buffer
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	buf.WriteString(`{"name":"insert","uuid":"`)
	// Use stack-allocated byte array for number formatting (no heap alloc)
	var numBuf [32]byte
	buf.Write(strconv.AppendUint(numBuf[:0], seq, 36))
	buf.WriteString(`","timestamp":`)
	buf.Write(strconv.AppendInt(numBuf[:0], timestamp, 10))
	buf.WriteString(`,"start_byte":0,"payload":`)
	if len(payload) > 0 {
		buf.Write(payload)
	} else {
		buf.WriteString(`null`)
	}
	buf.WriteString("}\n")

	// Single Write inside the lock — minimal critical section
	data := buf.Bytes()
	s.mu.Lock()
	_, _ = s.buffer.Write(data)
	s.mu.Unlock()

	bufferPool.Put(buf)
	return nil
}

func (s *JSONStorage) Close() error {
	s.closed.Store(true)

	s.mu.Lock()
	_ = s.buffer.Flush()
	s.mu.Unlock()

	return s.file.Close()
}

func (s *JSONStorage) Load() (<-chan LoadedCommand, <-chan error) {
	f, err := os.Open(s.Filename)
	if os.IsNotExist(err) {
		out := make(chan LoadedCommand)
		errs := make(chan error, 1)
		close(out)
		close(errs)
		return out, errs
	}
	if err != nil {
		out := make(chan LoadedCommand)
		errs := make(chan error, 1)
		errs <- err
		close(out)
		close(errs)
		return out, errs
	}

	out, errs := loadJSONCommands(f)
	wrappedOut := make(chan LoadedCommand, 100)
	wrappedErrs := make(chan error, 1)

	go func() {
		defer f.Close()
		defer close(wrappedOut)
		defer close(wrappedErrs)

		for cmd := range out {
			wrappedOut <- cmd
		}
		if err := <-errs; err != nil {
			wrappedErrs <- err
		}
	}()

	return wrappedOut, wrappedErrs
}
