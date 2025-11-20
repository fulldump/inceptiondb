package collection

import (
	"encoding/json"
	"sync"
	"testing"
)

func TestIndexMap_RemoveRow_Race(t *testing.T) {
	index := NewIndexMap(&IndexMapOptions{
		Field: "id",
	})

	var wg sync.WaitGroup
	const numGoroutines = 100

	// Populate initial data
	for i := 0; i < numGoroutines; i++ {
		data := map[string]interface{}{
			"id": "test-id",
		}
		payload, _ := json.Marshal(data)
		row := &Row{Payload: payload}
		_ = index.AddRow(row)
	}

	// Concurrently add and remove rows
	for i := 0; i < numGoroutines; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			data := map[string]interface{}{
				"id": "test-id",
			}
			payload, _ := json.Marshal(data)
			row := &Row{Payload: payload}
			_ = index.AddRow(row)
		}()

		go func() {
			defer wg.Done()
			data := map[string]interface{}{
				"id": "test-id",
			}
			payload, _ := json.Marshal(data)
			row := &Row{Payload: payload}
			_ = index.RemoveRow(row)
		}()
	}

	wg.Wait()
}
