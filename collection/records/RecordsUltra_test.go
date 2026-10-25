package records

import (
	"fmt"
	"runtime"
	"testing"
)

func Test_RecordsUltra_Suite(t *testing.T) {
	RunRecordsSuite(t, func() Records[int] {
		return NewRecordsUltra[int]()
	})
}

func Benchmark_RecordsUltra_Stress(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentMixedOperationsBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Insert(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentInsertBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Set(b *testing.B) {
	for _, workers := range []int{16, 32, 64, 128} {
		b.Run(fmt.Sprintf("workers_%d", workers), func(b *testing.B) {
			RunConcurrentSetBenchmark(b, workers, func() Records[int] {
				return NewRecordsUltra[int]()
			})
		})
	}
}

func Benchmark_RecordsUltra_Traverse(b *testing.B) {
	RunBenchmarkTraverse(b, func() Records[int] {
		return NewRecordsUltra[int]()
	})
}

// A single insert stream must spread over the shards, otherwise every later
// read and write of the collection contends on one shard lock.
func Test_RecordsUltra_SingleStreamUsesAllShards(t *testing.T) {
	r := NewRecordsUltra[int]()
	counts := map[int64]int{}
	for i := 0; i < recordsUltraNumShards*recordsUltraPickerRun; i++ {
		counts[r.Insert(i)&recordsUltraShardMask]++
	}
	if len(counts) < recordsUltraNumShards/2 {
		t.Fatalf("single stream used %d of %d shards", len(counts), recordsUltraNumShards)
	}
}

// Segments grow on demand, so ids set out of order with gaps (WAL replay)
// leave unallocated holes that every operation must handle.
func Test_RecordsUltra_SparseSet(t *testing.T) {
	r := NewRecordsUltra[int]()
	id := func(shard, lid int64) int64 { return lid<<recordsUltraShardBits | shard }

	want := map[int64]int{}
	for i, lid := range []int64{5000, 1, 4097, 20, 4095, 4096, 12345, 17} {
		want[id(int64(i%3), lid)] = i
		r.Set(id(int64(i%3), lid), i)
	}

	for k, v := range want {
		if got := r.Get(k); got != v {
			t.Fatalf("Get(%d) = %d, want %d", k, got, v)
		}
	}
	for _, missing := range []int64{id(0, 2), id(0, 4098), id(1, 9000), id(7, 1)} {
		if got := r.Get(missing); got != 0 {
			t.Fatalf("Get(%d) = %d, want 0", missing, got)
		}
		r.Delete(missing) // must not panic
	}

	seen := map[int64]int{}
	r.Traverse(func(k int64, v int) bool {
		seen[k] = v
		return true
	})
	if len(seen) != len(want) {
		t.Fatalf("Traverse returned %d records, want %d: %v", len(seen), len(want), seen)
	}
	for k, v := range want {
		if seen[k] != v {
			t.Fatalf("Traverse: id %d = %d, want %d", k, seen[k], v)
		}
	}

	// inserts after a sparse replay continue after the highest id of the shard
	for i := 0; i < 1000; i++ {
		k := r.Insert(1000 + i)
		if _, dup := want[k]; dup {
			t.Fatalf("Insert reused id %d", k)
		}
		want[k] = 1000 + i
	}
	for k, v := range want {
		if got := r.Get(k); got != v {
			t.Fatalf("after inserts Get(%d) = %d, want %d", k, got, v)
		}
	}
}

// A segment grows through every size up to recordsUltraSegmentSize and keeps
// its records while doing so.
func Test_RecordsUltra_SegmentGrowth(t *testing.T) {
	r := NewRecordsUltra[int]()
	ids := []int64{}
	for i := 0; i < 3*recordsUltraSegmentSize*recordsUltraNumShards/recordsUltraPickerRun; i++ {
		ids = append(ids, r.Insert(i))
	}
	for i, k := range ids {
		if got := r.Get(k); got != i {
			t.Fatalf("Get(%d) = %d, want %d", k, got, i)
		}
	}
	for _, shard := range r.shards {
		for i, seg := range shard.segments {
			if i < len(shard.segments)-1 && len(seg) != recordsUltraSegmentSize {
				t.Fatalf("shard %d segment %d has %d slots, want %d", shard.shardIndex, i, len(seg), recordsUltraSegmentSize)
			}
		}
	}
}

// Regression: every shard used to allocate a full segment upfront (~117MB
// per collection), which made thousands of small collections run out of memory.
func Test_RecordsUltra_SmallMemoryFootprint(t *testing.T) {
	heap := func() uint64 {
		runtime.GC()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		return m.HeapAlloc
	}

	const n = 20
	before := heap()
	rs := make([]*RecordsUltra[memTestRecord], n)
	for i := range rs {
		rs[i] = NewRecordsUltra[memTestRecord]()
		for j := 0; j < 100; j++ {
			rs[i].Insert(memTestRecord{})
		}
	}
	perRecords := (int64(heap()) - int64(before)) / n
	runtime.KeepAlive(rs)

	const limit = 1 << 20
	if perRecords > limit {
		t.Fatalf("RecordsUltra with 100 records uses %d bytes, want <= %d", perRecords, limit)
	}
	t.Logf("RecordsUltra with 100 records: %d KB", perRecords/1024)
}

// memTestRecord mimics the size of collection.Record without importing it.
type memTestRecord struct {
	Data   []byte
	Parsed any
	Active bool
}
