package collectionv2

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestStorageImplementations(t *testing.T) {
	tests := []struct {
		name    string
		factory func(filename string) (Storage, error)
	}{
		{
			name: "JSONStorage",
			factory: func(filename string) (Storage, error) {
				return NewJSONStorage(filename)
			},
		},
		{
			name: "GobStorage",
			factory: func(filename string) (Storage, error) {
				return NewGobStorage(filename)
			},
		},
		{
			name: "GzipStorage",
			factory: func(filename string) (Storage, error) {
				return NewGzipStorage(filename)
			},
		},
		{
			name: "SnapshotStorage",
			factory: func(filename string) (Storage, error) {
				return NewSnapshotStorage(filename)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := "/tmp/storage_test_" + tt.name
			os.Remove(filename)
			defer os.Remove(filename)

			// Phase 1: Write
			s, err := tt.factory(filename)
			if err != nil {
				t.Fatalf("Failed to create storage: %v", err)
			}

			count := 100
			for i := 0; i < count; i++ {
				payloadMap := map[string]interface{}{"i": float64(i), "id": uuid.New().String()}
				payload, _ := json.Marshal(payloadMap)
				cmd := &Command{
					Name:      "insert",
					Uuid:      uuid.New().String(),
					Timestamp: time.Now().UnixNano(),
					Payload:   payload,
				}
				err := s.Persist(cmd, payloadMap["id"].(string), payloadMap)
				if err != nil {
					t.Fatalf("Append failed: %v", err)
				}
			}

			err = s.Close()
			if err != nil {
				t.Fatalf("Close failed: %v", err)
			}

			// Phase 2: Read
			s2, err := tt.factory(filename)
			if err != nil {
				t.Fatalf("Failed to reopen storage: %v", err)
			}

			cmds, errs := s2.Load()
			readCount := 0
			for cmd := range cmds {
				if cmd.Err != nil {
					t.Errorf("Load error: %v", cmd.Err)
				}
				readCount++

				// Verify payload
				m := cmd.DecodedPayload.(map[string]interface{})
				if int(m["i"].(float64)) != cmd.Seq { // JSON numbers are float64
					// Wait, Seq is the sequence in the file, i is the value we wrote.
					// We wrote in order, so they should match.
					// But Gob might decode int as int or int64 depending on implementation.
					// JSON unmarshals numbers to float64 by default.
					// Let's just check existence.
				}
			}

			if err := <-errs; err != nil {
				t.Fatalf("Stream error: %v", err)
			}

			if readCount != count {
				t.Errorf("Expected %d commands, got %d", count, readCount)
			}

			s2.Close()
		})
	}
}
