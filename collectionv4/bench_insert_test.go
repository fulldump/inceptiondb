package collectionv4

import (
	"path"
	"testing"
	"time"
)

func BenchmarkCollectionInsert(b *testing.B) {
	filename := path.Join(b.TempDir(), "bench_data.wal")
	store, _ := NewJournal(filename)
	col := NewCollection("users", store)

	stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)
	payload := []byte(`{"name": "Test User", "email": "test@example.com", "active": true, "balance": 1500.50}`)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = col.Insert(payload)
	}

	b.StopTimer()
	close(stopFlusher)
	store.Close()
}
