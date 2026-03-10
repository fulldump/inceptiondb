package storage

import (
	"bufio"
	"fmt"
	"os"
	"sync"
)

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

	var err error
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	s.buffer = bufio.NewWriterSize(s.file, 16*1024*1024)

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
			buf := <-encodeCommandToBuffer(cmd)
			_, _ = s.buffer.Write(buf.Bytes())
			bufferPool.Put(buf)
		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					buf := <-encodeCommandToBuffer(cmd)
					_, _ = s.buffer.Write(buf.Bytes())
					bufferPool.Put(buf)
				default:
					return
				}
			}
		}
	}
}

func (s *JSONStorage) Persist(command *Command, id string, payload interface{}) error {
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
	_ = s.buffer.Flush()
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
