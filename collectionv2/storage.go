package collectionv2

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"
)

type Storage interface {
	// Persist persists a command.
	// id: the stable identifier of the row (if applicable, e.g. for insert/patch/remove)
	// payload: the current full value of the row (for insert/patch)
	Persist(cmd *Command, id string, payload interface{}) error
	Load() (<-chan LoadedCommand, <-chan error)
	Close() error
}

type LoadedCommand struct {
	Seq            int
	Cmd            *Command
	DecodedPayload interface{}
	Err            error
}

// --- JSONStorage ---

type JSONStorage struct {
	Filename     string
	file         *os.File
	buffer       *bufio.Writer
	commandQueue chan *Command
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func NewJSONStorage(filename string) (*JSONStorage, error) {
	s := &JSONStorage{
		Filename:     filename,
		commandQueue: make(chan *Command, 1000),
		closed:       make(chan struct{}),
	}

	// Open file for append
	var err error
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	s.buffer = bufio.NewWriterSize(s.file, 16*1024*1024)

	// Start background writer
	s.wg.Add(1)
	go s.writerLoop()

	return s, nil
}

func (s *JSONStorage) writerLoop() {
	defer s.wg.Done()
	for {
		select {
		case cmd, ok := <-s.commandQueue:
			if !ok {
				return
			}
			buf := <-cmd.serialized
			s.buffer.Write(buf.Bytes())
			bufferPool.Put(buf)

		case <-s.closed:
			// Drain queue
			for {
				select {
				case cmd := <-s.commandQueue:
					buf := <-cmd.serialized
					s.buffer.Write(buf.Bytes())
					bufferPool.Put(buf)
				default:
					return
				}
			}
		}
	}
}

func (s *JSONStorage) Persist(command *Command, id string, payload interface{}) error {
	command.serialized = make(chan *bytes.Buffer, 1)
	go func() {
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.Encode(command)

		command.serialized <- buf
	}()

	select {
	case s.commandQueue <- command:
		return nil
	case <-s.closed:
		return fmt.Errorf("storage closed")
	}
}

func (s *JSONStorage) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	s.wg.Wait()
	s.buffer.Flush()
	return s.file.Close()
}

func (s *JSONStorage) Load() (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		f, err := os.Open(s.Filename)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			errChan <- err
			return
		}
		defer f.Close()

		concurrency := runtime.NumCPU()

		// Reusing the logic from loader.go but adapted for JSONStorage
		// We need to implement the parallel loading here or call a helper.
		// Since I am refactoring, I will move the logic here.

		scanner := bufio.NewScanner(f)
		const maxCapacity = 16 * 1024 * 1024
		buf := make([]byte, maxCapacity)
		scanner.Buffer(buf, maxCapacity)

		lines := make(chan struct {
			seq  int
			data []byte
		}, 100)

		results := make(chan LoadedCommand, 100)

		var wg sync.WaitGroup
		for i := 0; i < concurrency; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for item := range lines {
					cmd := &Command{}
					err := json.Unmarshal(item.data, cmd)
					var decodedPayload interface{}
					if err == nil {
						switch cmd.Name {
						case "insert":
							m := map[string]interface{}{}
							err = json.Unmarshal(cmd.Payload, &m)
							decodedPayload = m
						case "remove":
							params := struct{ I int }{}
							err = json.Unmarshal(cmd.Payload, &params)
							decodedPayload = params
						case "patch":
							params := struct {
								I    int
								Diff map[string]interface{}
							}{}
							err = json.Unmarshal(cmd.Payload, &params)
							decodedPayload = params
						case "index":
							indexCommand := &CreateIndexCommand{}
							err = json.Unmarshal(cmd.Payload, indexCommand)
							decodedPayload = indexCommand
						case "drop_index":
							dropIndexCommand := &DropIndexCommand{}
							err = json.Unmarshal(cmd.Payload, dropIndexCommand)
							decodedPayload = dropIndexCommand
						case "set_defaults":
							defaults := map[string]any{}
							err = json.Unmarshal(cmd.Payload, &defaults)
							decodedPayload = defaults
						}
					}
					results <- LoadedCommand{
						Seq:            item.seq,
						Cmd:            cmd,
						DecodedPayload: decodedPayload,
						Err:            err,
					}
				}
			}()
		}

		// Feeder
		go func() {
			seq := 0
			for scanner.Scan() {
				data := make([]byte, len(scanner.Bytes()))
				copy(data, scanner.Bytes())
				lines <- struct {
					seq  int
					data []byte
				}{seq, data}
				seq++
			}
			close(lines)
			if err := scanner.Err(); err != nil {
				results <- LoadedCommand{Seq: -1, Err: err}
			}
			wg.Wait()
			close(results)
		}()

		// Re-assembler
		buffer := map[int]LoadedCommand{}
		nextSeq := 0

		for res := range results {
			if res.Err != nil {
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

// --- GobStorage ---

type GobStorage struct {
	Filename     string
	file         *os.File
	buffer       *bufio.Writer
	encoder      *gob.Encoder
	commandQueue chan *Command
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func NewGobStorage(filename string) (*GobStorage, error) {
	s := &GobStorage{
		Filename:     filename,
		commandQueue: make(chan *Command, 1000),
		closed:       make(chan struct{}),
	}

	var err error
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	s.buffer = bufio.NewWriterSize(s.file, 16*1024*1024)
	s.encoder = gob.NewEncoder(s.buffer)

	s.wg.Add(1)
	go s.writerLoop()

	return s, nil
}

func (s *GobStorage) writerLoop() {
	defer s.wg.Done()
	for {
		select {
		case cmd, ok := <-s.commandQueue:
			if !ok {
				return
			}
			// Gob encoder is not thread safe, so we encode here in the loop
			// This might be slower than JSON parallel encoding, but Gob is faster generally.
			err := s.encoder.Encode(cmd)
			if err != nil {
				// TODO: Handle error?
				fmt.Fprintf(os.Stderr, "Gob encode error: %v\n", err)
			}
			// We don't use serialized channel for Gob because we encode sequentially here

		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					s.encoder.Encode(cmd)
				default:
					return
				}
			}
		}
	}
}

func (s *GobStorage) Persist(command *Command, id string, payload interface{}) error {
	select {
	case s.commandQueue <- command:
		return nil
	case <-s.closed:
		return fmt.Errorf("storage closed")
	}
}

func (s *GobStorage) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	s.wg.Wait()
	s.buffer.Flush()
	return s.file.Close()
}

func (s *GobStorage) Load() (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		f, err := os.Open(s.Filename)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			errChan <- err
			return
		}
		defer f.Close()

		decoder := gob.NewDecoder(bufio.NewReader(f))

		seq := 0
		for {
			cmd := &Command{}
			err := decoder.Decode(cmd)
			if err == io.EOF {
				break
			}
			if err != nil {
				errChan <- err
				return
			}

			var decodedPayload interface{}
			switch cmd.Name {
			case "insert":
				m := map[string]interface{}{}
				json.Unmarshal(cmd.Payload, &m)
				decodedPayload = m
			case "remove":
				params := struct{ I int }{}
				json.Unmarshal(cmd.Payload, &params)
				decodedPayload = params
			case "patch":
				params := struct {
					I    int
					Diff map[string]interface{}
				}{}
				json.Unmarshal(cmd.Payload, &params)
				decodedPayload = params
			case "index":
				indexCommand := &CreateIndexCommand{}
				json.Unmarshal(cmd.Payload, indexCommand)
				decodedPayload = indexCommand
			case "drop_index":
				dropIndexCommand := &DropIndexCommand{}
				json.Unmarshal(cmd.Payload, dropIndexCommand)
				decodedPayload = dropIndexCommand
			case "set_defaults":
				defaults := map[string]any{}
				json.Unmarshal(cmd.Payload, &defaults)
				decodedPayload = defaults
			}

			out <- LoadedCommand{
				Seq:            seq,
				Cmd:            cmd,
				DecodedPayload: decodedPayload,
				Err:            nil,
			}
			seq++
		}
	}()

	return out, errChan
}

// --- GzipStorage ---

type GzipStorage struct {
	Filename   string
	file       *os.File
	gzipWriter *gzip.Writer
	buffer     *bufio.Writer // Buffer before gzip? Or gzip buffers itself? gzip buffers.
	// But we want to buffer writes to disk?
	// os.File -> bufio.Writer -> gzip.Writer -> json encoder

	commandQueue chan *Command
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func NewGzipStorage(filename string) (*GzipStorage, error) {
	s := &GzipStorage{
		Filename:     filename,
		commandQueue: make(chan *Command, 1000),
		closed:       make(chan struct{}),
	}

	var err error
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	// We buffer the file output
	s.buffer = bufio.NewWriterSize(s.file, 16*1024*1024)
	s.gzipWriter = gzip.NewWriter(s.buffer)

	s.wg.Add(1)
	go s.writerLoop()

	return s, nil
}

func (s *GzipStorage) writerLoop() {
	defer s.wg.Done()
	for {
		select {
		case cmd, ok := <-s.commandQueue:
			if !ok {
				return
			}
			buf := <-cmd.serialized
			s.gzipWriter.Write(buf.Bytes())
			bufferPool.Put(buf)

		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					buf := <-cmd.serialized
					s.gzipWriter.Write(buf.Bytes())
					bufferPool.Put(buf)
				default:
					return
				}
			}
		}
	}
}

func (s *GzipStorage) Persist(command *Command, id string, payload interface{}) error {
	command.serialized = make(chan *bytes.Buffer, 1)
	go func() {
		buf := bufferPool.Get().(*bytes.Buffer)
		buf.Reset()

		enc := json.NewEncoder(buf)
		enc.SetEscapeHTML(false)
		enc.Encode(command)

		command.serialized <- buf
	}()

	select {
	case s.commandQueue <- command:
		return nil
	case <-s.closed:
		return fmt.Errorf("storage closed")
	}
}

func (s *GzipStorage) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	s.wg.Wait()
	s.gzipWriter.Close()
	s.buffer.Flush()
	return s.file.Close()
}

func (s *GzipStorage) Load() (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		f, err := os.Open(s.Filename)
		if os.IsNotExist(err) {
			return
		}
		if err != nil {
			errChan <- err
			return
		}
		defer f.Close()

		gzipReader, err := gzip.NewReader(f)
		if err != nil {
			// If file is empty, gzip reader might fail?
			// If EOF, it's fine.
			if err == io.EOF {
				return
			}
			errChan <- err
			return
		}
		defer gzipReader.Close()

		// Reuse JSON loading logic?
		// We can refactor JSON loading to take an io.Reader
		// But for now, let's duplicate or adapt.

		scanner := bufio.NewScanner(gzipReader)
		const maxCapacity = 16 * 1024 * 1024
		buf := make([]byte, maxCapacity)
		scanner.Buffer(buf, maxCapacity)

		// ... (Same logic as JSONStorage.Load but reading from scanner)
		// To avoid duplication, we should extract the scanner loop.
		// But for this task, I'll just inline it to be safe and quick.

		seq := 0
		for scanner.Scan() {
			cmd := &Command{}
			err := json.Unmarshal(scanner.Bytes(), cmd)
			if err != nil {
				errChan <- err
				return
			}

			var decodedPayload interface{}
			switch cmd.Name {
			case "insert":
				m := map[string]interface{}{}
				json.Unmarshal(cmd.Payload, &m)
				decodedPayload = m
			case "remove":
				params := struct{ I int }{}
				json.Unmarshal(cmd.Payload, &params)
				decodedPayload = params
			case "patch":
				params := struct {
					I    int
					Diff map[string]interface{}
				}{}
				json.Unmarshal(cmd.Payload, &params)
				decodedPayload = params
			case "index":
				indexCommand := &CreateIndexCommand{}
				json.Unmarshal(cmd.Payload, indexCommand)
				decodedPayload = indexCommand
			case "drop_index":
				dropIndexCommand := &DropIndexCommand{}
				json.Unmarshal(cmd.Payload, dropIndexCommand)
				decodedPayload = dropIndexCommand
			case "set_defaults":
				defaults := map[string]any{}
				json.Unmarshal(cmd.Payload, &defaults)
				decodedPayload = defaults
			}

			out <- LoadedCommand{
				Seq:            seq,
				Cmd:            cmd,
				DecodedPayload: decodedPayload,
				Err:            nil,
			}
			seq++
		}

		if err := scanner.Err(); err != nil {
			errChan <- err
		}
	}()

	return out, errChan
}
