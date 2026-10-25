package collection

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"
)

func heapAlloc() int64 {
	runtime.GC()
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int64(m.HeapAlloc)
}

// Regression: every collection used to preallocate ~190MB (a 1M slot index
// queue, full records segments and async store queues), so a server with
// thousands of small collections ran out of memory while loading them.
func TestCollectionMemoryFootprint(t *testing.T) {
	const n = 20
	cases := []struct {
		name  string
		docs  int
		limit int64
	}{
		{name: "empty", docs: 0, limit: 512 << 10},
		{name: "one document", docs: 1, limit: 2 << 20}, // includes the 1MB WAL write buffer
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			before := heapAlloc()
			cols := make([]*Collection, n)
			for i := range cols {
				col, err := OpenCollection(filepath.Join(dir, fmt.Sprint(i)))
				if err != nil {
					t.Fatal(err)
				}
				for d := 0; d < tc.docs; d++ {
					if _, err := col.Insert([]byte(`{"a":1}`), false); err != nil {
						t.Fatal(err)
					}
				}
				cols[i] = col
			}
			perCollection := (heapAlloc() - before) / n
			for _, col := range cols {
				col.Close()
			}
			t.Logf("%s collection: %d KB", tc.name, perCollection/1024)
			if perCollection > tc.limit {
				t.Fatalf("%s collection uses %d bytes, want <= %d", tc.name, perCollection, tc.limit)
			}
		})
	}
}
