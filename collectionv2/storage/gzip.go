package storage

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"sync"
)

type GzipStorage struct {
	Filename     string
	file         *os.File
	gzipWriter   *gzip.Writer
	buffer       *bufio.Writer
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
	s.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

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
			buf := <-encodeCommandToBuffer(cmd)
			_, _ = s.gzipWriter.Write(buf.Bytes())
			bufferPool.Put(buf)
		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					buf := <-encodeCommandToBuffer(cmd)
					_, _ = s.gzipWriter.Write(buf.Bytes())
					bufferPool.Put(buf)
				default:
					return
				}
			}
		}
	}
}

func (s *GzipStorage) Persist(command *Command, id string, payload interface{}) error {
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
	_ = s.gzipWriter.Close()
	_ = s.buffer.Flush()
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
			if err == io.EOF {
				return
			}
			errChan <- err
			return
		}
		defer gzipReader.Close()

		loaded, loadedErrs := loadJSONCommands(gzipReader)
		for cmd := range loaded {
			out <- cmd
		}
		if err := <-loadedErrs; err != nil {
			errChan <- err
		}
	}()

	return out, errChan
}
