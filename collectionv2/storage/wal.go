package storage

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
)

const (
	walOpInsert uint8 = iota + 1
	walOpRemove
	walOpPatch
	walOpCreateIndex
	walOpDropIndex
	walOpSetDefaults
	walHeaderSize = 17
)

var walCRCTable = crc32.MakeTable(crc32.Castagnoli)

var walBufferPool = &sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 1024)
		return &b
	},
}

type WALStorage struct {
	Filename string
	file     *os.File
	writer   *bufio.Writer

	mu        sync.Mutex
	closed    atomic.Bool
	closeOnce sync.Once
	closeErr  error
}

func NewWALStorage(filename string) (*WALStorage, error) {
	f, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	return &WALStorage{
		Filename: filename,
		file:     f,
		writer:   bufio.NewWriterSize(f, 16*1024*1024),
	}, nil
}

func (s *WALStorage) Persist(command *Command, id string, payload interface{}) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	op, ok := commandNameToWALOp(command.Name)
	if !ok {
		return fmt.Errorf("unsupported command %q", command.Name)
	}

	data := command.Payload

	var header [walHeaderSize]byte
	header[0] = op
	binary.LittleEndian.PutUint64(header[1:9], uint64(parseWALID(id)))
	binary.LittleEndian.PutUint32(header[9:13], uint32(len(data)))
	binary.LittleEndian.PutUint32(header[13:17], crc32.Checksum(data, walCRCTable))

	bufPtr := walBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, header[:]...)
	buf = append(buf, data...)

	s.mu.Lock()
	_, err := s.writer.Write(buf)
	s.mu.Unlock()

	*bufPtr = buf
	walBufferPool.Put(bufPtr)

	return err
}

func (s *WALStorage) PersistInsert(seq uint64, timestamp int64, payload []byte) error {
	if s.closed.Load() {
		return fmt.Errorf("storage closed")
	}

	var header [walHeaderSize]byte
	header[0] = walOpInsert
	binary.LittleEndian.PutUint64(header[1:9], 0)
	binary.LittleEndian.PutUint32(header[9:13], uint32(len(payload)))
	binary.LittleEndian.PutUint32(header[13:17], crc32.Checksum(payload, walCRCTable))

	bufPtr := walBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, header[:]...)
	buf = append(buf, payload...)

	s.mu.Lock()
	_, err := s.writer.Write(buf)
	s.mu.Unlock()

	*bufPtr = buf
	walBufferPool.Put(bufPtr)

	return err
}

func (s *WALStorage) Load() (<-chan LoadedCommand, <-chan error) {
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

	out, errs := loadWALCommands(f)
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

func (s *WALStorage) Close() error {
	s.closeOnce.Do(func() {
		s.closed.Store(true)

		s.mu.Lock()
		defer s.mu.Unlock()

		if err := s.writer.Flush(); err != nil {
			s.closeErr = err
			return
		}
		if err := s.file.Sync(); err != nil {
			s.closeErr = err
			return
		}
		s.closeErr = s.file.Close()
	})

	return s.closeErr
}

func commandNameToWALOp(name string) (uint8, bool) {
	switch name {
	case "insert":
		return walOpInsert, true
	case "remove":
		return walOpRemove, true
	case "patch":
		return walOpPatch, true
	case "index":
		return walOpCreateIndex, true
	case "drop_index":
		return walOpDropIndex, true
	case "set_defaults":
		return walOpSetDefaults, true
	default:
		return 0, false
	}
}

func walOpToCommandName(op uint8) (string, bool) {
	switch op {
	case walOpInsert:
		return "insert", true
	case walOpRemove:
		return "remove", true
	case walOpPatch:
		return "patch", true
	case walOpCreateIndex:
		return "index", true
	case walOpDropIndex:
		return "drop_index", true
	case walOpSetDefaults:
		return "set_defaults", true
	default:
		return "", false
	}
}

func parseWALID(id string) int64 {
	if id == "" {
		return 0
	}
	v, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return 0
	}
	return v
}

func loadWALCommands(reader io.Reader) (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	type walFrame struct {
		seq         int
		op          uint8
		expectedCRC uint32
		data        []byte
	}

	go func() {
		defer close(out)
		defer close(errChan)

		concurrency := runtime.NumCPU()
		frames := make(chan walFrame, 100)
		results := make(chan LoadedCommand, 100)
		done := make(chan struct{})

		var stopOnce sync.Once
		stop := func() {
			stopOnce.Do(func() {
				close(done)
			})
		}

		var workerWG sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			workerWG.Add(1)
			go func() {
				defer workerWG.Done()
				for frame := range frames {
					actualCRC := crc32.Checksum(frame.data, walCRCTable)
					if actualCRC != frame.expectedCRC {
						select {
						case results <- LoadedCommand{
							Seq: frame.seq,
							Err: fmt.Errorf("WAL checksum mismatch: expected=%x got=%x", frame.expectedCRC, actualCRC),
						}:
						case <-done:
						}
						continue
					}

					name, ok := walOpToCommandName(frame.op)
					if !ok {
						select {
						case results <- LoadedCommand{Seq: frame.seq, Err: fmt.Errorf("unknown WAL operation code %d", frame.op)}:
						case <-done:
						}
						continue
					}

					cmd := &Command{
						Name:    name,
						Payload: frame.data,
					}
					decodedPayload, err := decodePayload(cmd)

					select {
					case results <- LoadedCommand{
						Seq:            frame.seq,
						Cmd:            cmd,
						DecodedPayload: decodedPayload,
						Err:            err,
					}:
					case <-done:
						return
					}
				}
			}()
		}

		go func() {
			defer close(frames)

			walReader := bufio.NewReaderSize(reader, 16*1024*1024)
			header := make([]byte, walHeaderSize)
			seq := 0

			for {
				_, err := io.ReadFull(walReader, header)
				if err != nil {
					if errors.Is(err, io.EOF) {
						return
					}
					if errors.Is(err, io.ErrUnexpectedEOF) {
						select {
						case results <- LoadedCommand{Seq: -1, Err: fmt.Errorf("unexpected EOF while reading WAL header")}:
						case <-done:
						}
						return
					}
					select {
					case results <- LoadedCommand{Seq: -1, Err: err}:
					case <-done:
					}
					return
				}

				length := binary.LittleEndian.Uint32(header[9:13])
				expectedCRC := binary.LittleEndian.Uint32(header[13:17])

				payload := make([]byte, length)
				if length > 0 {
					if _, err := io.ReadFull(walReader, payload); err != nil {
						select {
						case results <- LoadedCommand{Seq: -1, Err: fmt.Errorf("unexpected EOF while reading WAL payload: %w", err)}:
						case <-done:
						}
						return
					}
				}

				select {
				case frames <- walFrame{seq: seq, op: header[0], expectedCRC: expectedCRC, data: payload}:
				case <-done:
					return
				}
				seq++
			}
		}()

		go func() {
			workerWG.Wait()
			close(results)
		}()

		buffer := map[int]LoadedCommand{}
		nextSeq := 0

		for res := range results {
			if res.Err != nil {
				stop()
				errChan <- res.Err
				return
			}

			if res.Seq == nextSeq {
				out <- res
				nextSeq++

				for {
					if cmd, ok := buffer[nextSeq]; ok {
						delete(buffer, nextSeq)
						out <- cmd
						nextSeq++
					} else {
						break
					}
				}
			} else {
				buffer[res.Seq] = res
			}
		}
	}()

	return out, errChan
}
