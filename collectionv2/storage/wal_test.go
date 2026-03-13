package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWALStorageSuite(t *testing.T) {
	runStorageSuite(t, func(filename string) (Storage, error) {
		return NewWALStorage(filename)
	})
}

func TestWALStorageDetectsCorruption(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "wal_storage_corruption")

	s, err := NewWALStorage(filename)
	if err != nil {
		t.Fatalf("create WAL storage: %v", err)
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"id": "row-1",
		"i":  1.0,
	})
	cmd := &Command{
		Name:      "insert",
		Uuid:      "abc",
		Timestamp: time.Now().UnixNano(),
		Payload:   payload,
	}
	if err := s.Persist(cmd, "1", nil); err != nil {
		t.Fatalf("persist command: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close storage: %v", err)
	}

	f, err := os.OpenFile(filename, os.O_RDWR, 0o666)
	if err != nil {
		t.Fatalf("open WAL file: %v", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		t.Fatalf("stat WAL file: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("unexpected empty WAL file")
	}

	_, err = f.Seek(-1, 2)
	if err != nil {
		t.Fatalf("seek WAL file: %v", err)
	}

	last := []byte{0}
	if _, err := f.Read(last); err != nil {
		t.Fatalf("read WAL tail: %v", err)
	}

	if _, err := f.Seek(-1, 2); err != nil {
		t.Fatalf("seek WAL file for write: %v", err)
	}
	last[0] ^= 0xFF
	if _, err := f.Write(last); err != nil {
		t.Fatalf("write WAL tail: %v", err)
	}

	s2, err := NewWALStorage(filename)
	if err != nil {
		t.Fatalf("reopen WAL storage: %v", err)
	}
	defer s2.Close()

	cmds, errs := s2.Load()
	for range cmds {
	}

	if err := <-errs; err == nil {
		t.Fatalf("expected corruption error, got nil")
	}
}
