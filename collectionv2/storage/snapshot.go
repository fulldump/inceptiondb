package storage

import (
	"bufio"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// SnapshotStorage implements Storage with snapshotting and WAL.
// It keeps the entire state in memory and periodically persists it to a snapshot file.
// Between snapshots, operations are appended to a Write-Ahead Log (WAL).
type SnapshotStorage struct {
	Filename string
	WalFile  *os.File
	WalBuf   *bufio.Writer

	Rows     map[string]interface{}
	Indexes  map[string]*CreateIndexCommand
	Defaults map[string]interface{}

	WalCount     int
	WalThreshold int

	mutex sync.RWMutex

	commandQueue chan *Command
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

func NewSnapshotStorage(filename string) (*SnapshotStorage, error) {
	s := &SnapshotStorage{
		Filename:     filename,
		Rows:         make(map[string]interface{}),
		Indexes:      make(map[string]*CreateIndexCommand),
		Defaults:     make(map[string]interface{}),
		WalThreshold: 1000,
		commandQueue: make(chan *Command, 1000),
		closed:       make(chan struct{}),
	}

	var err error
	s.WalFile, err = os.OpenFile(filename+".wal", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o666)
	if err != nil {
		return nil, fmt.Errorf("open wal file: %w", err)
	}
	s.WalBuf = bufio.NewWriterSize(s.WalFile, 16*1024*1024)

	s.wg.Add(1)
	go s.writerLoop()

	return s, nil
}

func (s *SnapshotStorage) writerLoop() {
	defer s.wg.Done()
	for {
		select {
		case cmd, ok := <-s.commandQueue:
			if !ok {
				return
			}
			s.handleCommand(cmd)
		case <-s.closed:
			for {
				select {
				case cmd := <-s.commandQueue:
					s.handleCommand(cmd)
				default:
					return
				}
			}
		}
	}
}

func (s *SnapshotStorage) handleCommand(cmd *Command) {
	buf := <-encodeCommandToBuffer(cmd)
	_, _ = s.WalBuf.Write(buf.Bytes())
	bufferPool.Put(buf)

	s.WalCount++
	if s.WalCount >= s.WalThreshold {
		s.snapshot()
	}
}

func (s *SnapshotStorage) snapshot() {
	_ = s.WalBuf.Flush()

	snapFile, err := os.Create(s.Filename + ".snap.tmp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "snapshot create error: %v\n", err)
		return
	}
	defer snapFile.Close()

	enc := gob.NewEncoder(snapFile)

	s.mutex.RLock()
	err = enc.Encode(s.Rows)
	if err == nil {
		err = enc.Encode(s.Indexes)
	}
	if err == nil {
		err = enc.Encode(s.Defaults)
	}
	s.mutex.RUnlock()

	if err != nil {
		fmt.Fprintf(os.Stderr, "snapshot encode error: %v\n", err)
		return
	}

	if err := os.Rename(s.Filename+".snap.tmp", s.Filename+".snap"); err != nil {
		fmt.Fprintf(os.Stderr, "snapshot rename error: %v\n", err)
		return
	}

	_ = s.WalFile.Truncate(0)
	_, _ = s.WalFile.Seek(0, 0)
	s.WalBuf.Reset(s.WalFile)
	s.WalCount = 0
}

func (s *SnapshotStorage) Persist(command *Command, id string, payload interface{}) error {
	s.mutex.Lock()
	switch command.Name {
	case "insert", "patch":
		s.Rows[id] = payload
	case "remove":
		delete(s.Rows, id)
	case "index":
		var idxCmd CreateIndexCommand
		_ = json.Unmarshal(command.Payload, &idxCmd)
		s.Indexes[idxCmd.Name] = &idxCmd
	case "drop_index":
		var dropCmd DropIndexCommand
		_ = json.Unmarshal(command.Payload, &dropCmd)
		delete(s.Indexes, dropCmd.Name)
	case "set_defaults":
		var defaults map[string]interface{}
		_ = json.Unmarshal(command.Payload, &defaults)
		s.Defaults = defaults
	}
	s.mutex.Unlock()

	var walCmd *Command
	switch command.Name {
	case "insert", "patch":
		p, _ := json.Marshal(payload)
		walCmd = &Command{
			Name:      "set",
			Payload:   p,
			Timestamp: command.Timestamp,
			Uuid:      command.Uuid,
		}
	case "remove":
		p, _ := json.Marshal(map[string]string{"id": id})
		walCmd = &Command{
			Name:      "delete",
			Payload:   p,
			Timestamp: command.Timestamp,
			Uuid:      command.Uuid,
		}
	default:
		walCmd = command
	}

	select {
	case s.commandQueue <- walCmd:
		return nil
	case <-s.closed:
		return fmt.Errorf("storage closed")
	}
}

func (s *SnapshotStorage) Close() error {
	s.closeOnce.Do(func() {
		close(s.closed)
	})
	s.wg.Wait()
	_ = s.WalBuf.Flush()
	return s.WalFile.Close()
}

func (s *SnapshotStorage) Load() (<-chan LoadedCommand, <-chan error) {
	out := make(chan LoadedCommand, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(out)
		defer close(errChan)

		f, err := os.Open(s.Filename + ".snap")
		if err == nil {
			defer f.Close()
			dec := gob.NewDecoder(f)

			var rows map[string]interface{}
			var indexes map[string]*CreateIndexCommand
			var defaults map[string]interface{}

			if err := dec.Decode(&rows); err == nil {
				s.Rows = rows
			}
			if err := dec.Decode(&indexes); err == nil {
				s.Indexes = indexes
			}
			if err := dec.Decode(&defaults); err == nil {
				s.Defaults = defaults
			}
		}

		walFile, err := os.Open(s.Filename + ".wal")
		if err == nil {
			defer walFile.Close()

			scanner := bufio.NewScanner(walFile)
			const maxCapacity = 16 * 1024 * 1024
			buf := make([]byte, maxCapacity)
			scanner.Buffer(buf, maxCapacity)

			for scanner.Scan() {
				cmd := &Command{}
				if err := json.Unmarshal(scanner.Bytes(), cmd); err != nil {
					continue
				}

				switch cmd.Name {
				case "set":
					m := map[string]interface{}{}
					_ = json.Unmarshal(cmd.Payload, &m)
					if id, ok := m["id"].(string); ok {
						s.Rows[id] = m
					}
				case "delete":
					m := map[string]string{}
					_ = json.Unmarshal(cmd.Payload, &m)
					if id, ok := m["id"]; ok {
						delete(s.Rows, id)
					}
				case "index":
					indexCommand := &CreateIndexCommand{}
					_ = json.Unmarshal(cmd.Payload, indexCommand)
					s.Indexes[indexCommand.Name] = indexCommand
				case "drop_index":
					dropIndexCommand := &DropIndexCommand{}
					_ = json.Unmarshal(cmd.Payload, dropIndexCommand)
					delete(s.Indexes, dropIndexCommand.Name)
				case "set_defaults":
					defaults := map[string]any{}
					_ = json.Unmarshal(cmd.Payload, &defaults)
					s.Defaults = defaults
				}
			}
		}

		if len(s.Defaults) > 0 {
			out <- LoadedCommand{
				Cmd:            &Command{Name: "set_defaults"},
				DecodedPayload: s.Defaults,
			}
		}

		for _, idx := range s.Indexes {
			out <- LoadedCommand{
				Cmd:            &Command{Name: "index"},
				DecodedPayload: idx,
			}
		}

		seq := 0
		for _, row := range s.Rows {
			out <- LoadedCommand{
				Seq:            seq,
				Cmd:            &Command{Name: "insert"},
				DecodedPayload: row,
			}
			seq++
		}
	}()

	return out, errChan
}
