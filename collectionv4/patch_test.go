package collectionv4

import (
	"path"
	"testing"
)

func BenchmarkPatch(b *testing.B) {
	// Setup
	filename := path.Join(b.TempDir(), "bench.wal")
	store, _ := NewStoreDisk(filename)
	col := NewCollection("bench", store)

	payload := []byte(`{"id": 1, "name": "Alice Wonderland", "email": "alice@example.com", "age": 30, "active": true, "balance": 1500.50, "tags": ["premium", "user"], "address": {"city": "Madrid", "zip": "28080"}}`)
	id, _ := col.Insert(payload, false)

	patch := map[string]interface{}{
		"age":     31,
		"balance": 1550.75,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// apply patch
		err := col.Patch(id, patch, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}
