package storage

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
)

func runStorageSuite(t *testing.T, factory func(filename string) (Storage, error)) {
	t.Helper()

	base := filepath.Join(t.TempDir(), "storage_test")

	s, err := factory(base)
	if err != nil {
		t.Fatalf("failed to create storage: %v", err)
	}

	const count = 100
	for i := 0; i < count; i++ {
		payloadMap := map[string]interface{}{
			"i":  float64(i),
			"id": uuid.New().String(),
		}
		payload, _ := json.Marshal(payloadMap)
		cmd := &Command{
			Name:      "insert",
			Uuid:      uuid.New().String(),
			Timestamp: time.Now().UnixNano(),
			Payload:   payload,
		}
		if err := s.Persist(cmd, payloadMap["id"].(string), payloadMap); err != nil {
			t.Fatalf("persist failed: %v", err)
		}
	}

	if err := s.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	s2, err := factory(base)
	if err != nil {
		t.Fatalf("failed to reopen storage: %v", err)
	}
	defer s2.Close()

	cmds, errs := s2.Load()
	readCount := 0
	for cmd := range cmds {
		if cmd.Err != nil {
			t.Fatalf("load command error: %v", cmd.Err)
		}
		if cmd.Cmd == nil {
			t.Fatalf("loaded command is nil")
		}
		if cmd.Cmd.Name == "insert" {
			if _, ok := cmd.DecodedPayload.(map[string]interface{}); !ok {
				t.Fatalf("unexpected payload type: %T", cmd.DecodedPayload)
			}
		}
		readCount++
	}

	if err := <-errs; err != nil {
		t.Fatalf("load stream error: %v", err)
	}

	if readCount != count {
		t.Fatalf("expected %d commands, got %d", count, readCount)
	}

	_ = os.Remove(base)
	_ = os.Remove(base + ".wal")
	_ = os.Remove(base + ".snap")
}
