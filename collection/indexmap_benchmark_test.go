package collection

import (
	"fmt"
	"math/rand"
	"testing"
)

func BenchmarkIndexMap_RemoveRow_Concurrent(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexMap(options)

	// Pre-populate the index with a large number of items to simulate a real scenario
	const initialSize = 100000
	for i := 0; i < initialSize; i++ {
		key := fmt.Sprintf("key-%d", i)
		row := &Row{Payload: createPayload(key, options.Field)}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// Generate a random key to remove to simulate random access
			// We use a key range slightly larger than initialSize to also test "not found" cases occasionally,
			// but mostly we want to test removal contention.
			// However, if we only remove, we will run out of items.
			// So we will randomly add or remove to keep the map populated,
			// OR we can just accept that we are removing items and the map shrinks.
			// For a pure "RemoveRow" benchmark, we should probably just try to remove existing items.
			// But if we remove them all, the benchmark becomes trivial.
			//
			// To strictly test RemoveRow with mutex contention, we can try to remove random keys.
			// Even if they don't exist, the mutex is still acquired to check.
			//
			// Let's try to remove keys from the initial set.

			i := rand.Intn(initialSize)
			key := fmt.Sprintf("key-%d", i)
			row := &Row{Payload: createPayload(key, options.Field)}

			// We ignore the error because the item might have been already removed by another goroutine
			_ = index.RemoveRow(row)
		}
	})
}
