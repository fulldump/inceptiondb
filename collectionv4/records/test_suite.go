package records

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// RunRecordsSuite ejecuta una batería de tests reutilizable sobre cualquier
// implementación de Records[int].
//
// Usamos int para que la suite sea fácil de reutilizar y podamos comprobar
// igualdad directamente sin meter comparadores adicionales.
// Si alguna implementación es genérica, simplemente instánciala como Records[int].
func RunRecordsSuite(t *testing.T, newRecords func() Records[int]) {
	t.Helper()

	t.Run("insert_then_get", func(t *testing.T) {
		r := newRecords()

		id := r.Insert(123)
		got := r.Get(id)

		if got != 123 {
			t.Fatalf("Get(%d) = %d, want %d", id, got, 123)
		}
	})

	t.Run("delete_removes_value", func(t *testing.T) {
		r := newRecords()

		id := r.Insert(456)
		if got := r.Get(id); got != 456 {
			t.Fatalf("Get(%d) before delete = %d, want %d", id, got, 456)
		}

		r.Delete(id)

		// La interfaz no devuelve bool ni error, así que asumimos el contrato
		// implícito habitual: tras borrar, Get devuelve el zero value.
		if got := r.Get(id); got != 0 {
			t.Fatalf("Get(%d) after delete = %d, want zero value", id, got)
		}
	})

	t.Run("ids_are_unique_and_monotonic_sequentially", func(t *testing.T) {
		t.Skip("it is not a requirement")
		r := newRecords()

		id1 := r.Insert(10)
		id2 := r.Insert(20)
		id3 := r.Insert(30)

		if id1 <= 0 {
			t.Fatalf("first id = %d, want > 0", id1)
		}
		if id2 != id1+1 {
			t.Fatalf("second id = %d, want %d", id2, id1+1)
		}
		if id3 != id2+1 {
			t.Fatalf("third id = %d, want %d", id3, id2+1)
		}
	})

	t.Run("concurrent_inserts_return_unique_ids_and_preserve_values", func(t *testing.T) {
		t.Skip("ids can be reused")
		r := newRecords()

		const n = 2000

		type pair struct {
			id  int64
			val int
		}

		results := make(chan pair, n)
		var wg sync.WaitGroup

		for i := 1; i <= n; i++ {
			wg.Add(1)
			val := i
			go func() {
				defer wg.Done()
				id := r.Insert(val)
				results <- pair{id: id, val: val}
			}()
		}

		wg.Wait()
		close(results)

		seenIDs := make(map[int64]int, n)

		for p := range results {
			if p.id <= 0 {
				t.Fatalf("Insert(%d) returned invalid id %d", p.val, p.id)
			}
			if prev, exists := seenIDs[p.id]; exists {
				t.Fatalf("duplicate id detected: %d used for values %d and %d", p.id, prev, p.val)
			}
			seenIDs[p.id] = p.val
		}

		if len(seenIDs) != n {
			t.Fatalf("got %d unique ids, want %d", len(seenIDs), n)
		}

		for id, want := range seenIDs {
			got := r.Get(id)
			if got != want {
				t.Fatalf("Get(%d) = %d, want %d", id, got, want)
			}
		}
	})

	t.Run("concurrent_readers_on_same_record", func(t *testing.T) {
		r := newRecords()

		id := r.Insert(999)

		const readers = 128
		const iterations = 2000

		var wg sync.WaitGroup
		for i := 0; i < readers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < iterations; j++ {
					got := r.Get(id)
					if got != 999 {
						t.Errorf("Get(%d) = %d, want %d", id, got, 999)
						return
					}
				}
			}()
		}
		wg.Wait()
	})
}

func RunConcurrentMixedOperationsBenchmark(b *testing.B, workers int, newRecords func() Records[int]) {
	b.Helper()

	r := newRecords()

	var nextVal atomic.Int64
	nextVal.Store(10000)

	for i := 1; i <= 500; i++ {
		r.Insert(i)
	}

	var maxID atomic.Int64
	maxID.Store(500)

	var insertOps atomic.Int64
	var getOps atomic.Int64
	var deleteOps atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			for i := worker; i < b.N; i += workers {
				switch i % 3 {
				case 0:
					insertOps.Add(1)
					val := int(nextVal.Add(1))
					id := r.Insert(val)
					for {
						curr := maxID.Load()
						if id <= curr || maxID.CompareAndSwap(curr, id) {
							break
						}
					}
				case 1:
					getOps.Add(1)
					limit := maxID.Load()
					if limit > 0 {
						id := int64((worker+i)%int(limit) + 1)
						_ = r.Get(id)
					}
				case 2:
					deleteOps.Add(1)
					limit := maxID.Load()
					if limit > 0 {
						id := int64((worker*31+i)%int(limit) + 1)
						r.Delete(id)
					}
				}
			}
		}(w)
	}

	wg.Wait()

	elapsed := time.Since(start)
	totalOps := insertOps.Load() + getOps.Load() + deleteOps.Load()
	opsPerSec := float64(totalOps) / elapsed.Seconds()
	secPerMillionOps := elapsed.Seconds() / (float64(totalOps) / 1_000_000)
	b.ReportMetric(float64(insertOps.Load()), "insert_total")
	b.ReportMetric(float64(getOps.Load()), "get_total")
	b.ReportMetric(float64(deleteOps.Load()), "delete_total")
	b.ReportMetric(float64(elapsed.Milliseconds()), "elapsed_ms")
	b.ReportMetric(opsPerSec, "ops_per_sec")
	// b.ReportMetric(secPerMillionOps, "sec_per_million_ops")
	b.ReportMetric(1/secPerMillionOps, "M/s")
	b.Logf(
		"workers=%d elapsed=%s insert=%d get=%d delete=%d ops/s=%.0f M/s=%.6f",
		workers,
		elapsed,
		insertOps.Load(),
		getOps.Load(),
		deleteOps.Load(),
		opsPerSec,
		1/secPerMillionOps,
	)
}

func RunConcurrentInsertBenchmark(b *testing.B, workers int, newRecords func() Records[int]) {
	b.Helper()

	r := newRecords()

	var nextVal atomic.Int64
	nextVal.Store(0)

	var insertOps atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			for i := worker; i < b.N; i += workers {
				insertOps.Add(1)
				val := int(nextVal.Add(1))
				r.Insert(val)
			}
		}(w)
	}

	wg.Wait()

	elapsed := time.Since(start)
	totalOps := insertOps.Load()
	opsPerSec := float64(totalOps) / elapsed.Seconds()
	secPerMillionOps := elapsed.Seconds() / (float64(totalOps) / 1_000_000)
	b.ReportMetric(float64(insertOps.Load()), "insert_total")
	b.ReportMetric(float64(elapsed.Milliseconds()), "elapsed_ms")
	b.ReportMetric(opsPerSec, "ops_per_sec")
	b.ReportMetric(1/secPerMillionOps, "M/s")
	b.Logf(
		"workers=%d elapsed=%s insert=%d ops/s=%.0f M/s=%.6f",
		workers,
		elapsed,
		insertOps.Load(),
		opsPerSec,
		1/secPerMillionOps,
	)
}

func RunConcurrentSetBenchmark(b *testing.B, workers int, newRecords func() Records[int]) {
	b.Helper()

	r := newRecords()

	var setOps atomic.Int64

	b.ReportAllocs()
	b.ResetTimer()
	start := time.Now()

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()

			for i := worker; i < b.N; i += workers {
				setOps.Add(1)
				// Using index + 1 ensures we avoid id=0, which is invalid for some implementations
				r.Set(int64(i+1), i)
			}
		}(w)
	}

	wg.Wait()

	elapsed := time.Since(start)
	totalOps := setOps.Load()
	opsPerSec := float64(totalOps) / elapsed.Seconds()
	secPerMillionOps := elapsed.Seconds() / (float64(totalOps) / 1_000_000)
	b.ReportMetric(float64(setOps.Load()), "set_total")
	b.ReportMetric(float64(elapsed.Milliseconds()), "elapsed_ms")
	b.ReportMetric(opsPerSec, "ops_per_sec")
	b.ReportMetric(1/secPerMillionOps, "M/s")
	b.Logf(
		"workers=%d elapsed=%s set=%d ops/s=%.0f M/s=%.6f",
		workers,
		elapsed,
		setOps.Load(),
		opsPerSec,
		1/secPerMillionOps,
	)
}
