package collectionv4

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
)

var storeJsonBufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type StoreJson struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
	closed atomic.Bool
}

func NewStoreJson(path string) (*StoreJson, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	s := &StoreJson{
		file:   f,
		writer: bufio.NewWriterSize(f, 4*1024*1024), // 4MB buffer for good throughput
	}
	
	return s, nil
}

func (s *StoreJson) Append(op uint8, id int64, data []byte, sync bool) error {
	if s.closed.Load() {
		return fmt.Errorf("StoreJson closed")
	}

	buf := storeJsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	buf.WriteString(`{"op":`)
	
	// Use stack-allocated byte array for formatting 
	var numBuf [32]byte
	buf.Write(strconv.AppendUint(numBuf[:0], uint64(op), 10))
	
	buf.WriteString(`,"id":`)
	buf.Write(strconv.AppendInt(numBuf[:0], id, 10))

	if len(data) > 0 {
		buf.WriteString(`,"data":`)
		buf.Write(data)
	}

	buf.WriteString("}\n")

	finalData := buf.Bytes()
	
	// Minimal critical section to write to our buffer
	s.mu.Lock()
	_, err := s.writer.Write(finalData)
	s.mu.Unlock()

	storeJsonBufferPool.Put(buf)
	
	if sync {
		if err := s.writer.Flush(); err != nil {
			return err
		}
		if err := s.file.Sync(); err != nil {
			return err
		}
	}
	
	return err
}

func (s *StoreJson) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return fmt.Errorf("StoreJson closed")
	}
	return s.writer.Flush()
}

func (s *StoreJson) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed.Load() {
		return fmt.Errorf("StoreJson closed")
	}
	if err := s.writer.Flush(); err != nil {
		return err
	}
	return s.file.Sync()
}

func (s *StoreJson) Close() error {
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

func (s *StoreJson) Replay(fn func(op uint8, id int64, data []byte) error) error {
	f, err := os.Open(s.file.Name())
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	// Some JSON payloads might be huge, allocate a 64MB buffer for reading long lines max
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 64*1024*1024)

	type LogLine struct {
		Op   uint8           `json:"op"`
		ID   int64           `json:"id"`
		Data json.RawMessage `json:"data,omitempty"`
	}

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var parsedLog LogLine
		if err := json.Unmarshal(line, &parsedLog); err != nil {
			return fmt.Errorf("StoreJson Replay JSON error: %w (line ID: maybe %v)", err, line[:10]) // Provide partial line for debug
		}

		// Important: If data was extracted, we pass the raw bytes. If not, we pass nil
		var finalData []byte
		if len(parsedLog.Data) > 0 {
			finalData = []byte(parsedLog.Data)
		}

		if err := fn(parsedLog.Op, parsedLog.ID, finalData); err != nil {
			return err
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	return nil
}
