package storage

import (
	"bufio"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"sync"
)

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
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
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
			_ = s.encoder.Encode(cmd)
		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					_ = s.encoder.Encode(cmd)
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
	_ = s.buffer.Flush()
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

			decodedPayload, err := decodePayload(cmd)
			if err != nil {
				errChan <- err
				return
			}

			out <- LoadedCommand{
				Seq:            seq,
				Cmd:            cmd,
				DecodedPayload: decodedPayload,
			}
			seq++
		}
	}()

	return out, errChan
}
