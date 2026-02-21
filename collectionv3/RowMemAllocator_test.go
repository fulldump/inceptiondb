package collectionv3

import (
	"math/rand"
	"testing"
)

func TestMemAllocator_BasicAllocGet(t *testing.T) {
	a := NewMemAllocator[int]()

	id := a.Alloc(42)
	if id != 0 {
		t.Fatalf("expected first id=0, got %d", id)
	}

	v, ok := a.Get(id)
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if v != 42 {
		t.Fatalf("expected value=42, got %d", v)
	}

	_, ok = a.Get(999)
	if ok {
		t.Fatalf("expected ok=false for out-of-range id")
	}
}

func TestMemAllocator_FreeMakesGetFail(t *testing.T) {
	a := NewMemAllocator[string]()

	id := a.Alloc("hello")
	if v, ok := a.Get(id); !ok || v != "hello" {
		t.Fatalf("expected to read alive value")
	}

	a.Free(id)

	if _, ok := a.Get(id); ok {
		t.Fatalf("expected Get to fail after Free")
	}
}

func TestMemAllocator_ReusesFreedSlots_LIFO(t *testing.T) {
	a := NewMemAllocator[int]()

	id0 := a.Alloc(10) // 0
	id1 := a.Alloc(11) // 1
	id2 := a.Alloc(12) // 2

	if id0 != 0 || id1 != 1 || id2 != 2 {
		t.Fatalf("unexpected ids: got %d,%d,%d", id0, id1, id2)
	}

	// Free two slots; free-list is a stack (LIFO).
	a.Free(id1) // push 1
	a.Free(id2) // push 2

	// Next alloc should reuse id2 first (LIFO)
	idA := a.Alloc(99)
	if idA != id2 {
		t.Fatalf("expected reuse id=%d (LIFO), got %d", id2, idA)
	}

	// Next alloc should reuse id1
	idB := a.Alloc(100)
	if idB != id1 {
		t.Fatalf("expected reuse id=%d (LIFO), got %d", id1, idB)
	}

	// Values should match
	if v, ok := a.Get(idA); !ok || v != 99 {
		t.Fatalf("expected idA value=99 ok=true, got v=%d ok=%v", v, ok)
	}
	if v, ok := a.Get(idB); !ok || v != 100 {
		t.Fatalf("expected idB value=100 ok=true, got v=%d ok=%v", v, ok)
	}
}

func TestMemAllocator_GrowsOnlyWhenNoFreeSlots(t *testing.T) {
	a := NewMemAllocator[int]()

	_ = a.Alloc(1)
	_ = a.Alloc(2)
	_ = a.Alloc(3)

	if got := len(a.rows); got != 3 {
		t.Fatalf("expected rows len=3, got %d", got)
	}

	// Free one; length should not shrink.
	a.Free(1)
	if got := len(a.rows); got != 3 {
		t.Fatalf("expected rows len still 3 after Free, got %d", got)
	}

	// Next alloc should reuse freed slot; still no growth.
	_ = a.Alloc(99)
	if got := len(a.rows); got != 3 {
		t.Fatalf("expected rows len still 3 after reuse, got %d", got)
	}

	// Now alloc again; no free slots, should grow.
	_ = a.Alloc(100)
	if got := len(a.rows); got != 4 {
		t.Fatalf("expected rows len=4 after growth, got %d", got)
	}
}

func TestMemAllocator_FreeIsIdempotentAndSafe(t *testing.T) {
	a := NewMemAllocator[int]()

	id := a.Alloc(123)
	a.Free(id)

	// Free again (should be safe / no panic / no double free effect)
	a.Free(id)

	// Free invalid ids should be safe
	a.Free(-1)
	a.Free(999)

	// The slot should still be empty
	if _, ok := a.Get(id); ok {
		t.Fatalf("expected Get fail after Free")
	}

	// Alloc should reuse the freed slot once (even if Free was called twice)
	id2 := a.Alloc(7)
	if id2 != id {
		t.Fatalf("expected reuse id=%d, got %d", id, id2)
	}

	// Next alloc should NOT reuse id again; it should grow to 1 (since we started with 1 slot)
	id3 := a.Alloc(8)
	if id3 == id {
		t.Fatalf("unexpected reuse of the same id twice; likely double-free bug")
	}
}

func TestMemAllocator_ZeroValueClearingHelpsGCForPointers(t *testing.T) {
	type Big struct {
		Data []byte
	}
	a := NewMemAllocator[*Big]()

	obj := &Big{Data: make([]byte, 1024)}
	id := a.Alloc(obj)

	// Ensure stored
	got, ok := a.Get(id)
	if !ok || got == nil || len(got.Data) != 1024 {
		t.Fatalf("expected stored pointer to be retrievable")
	}

	// Free should clear to zero value (nil pointer) in the slot.
	a.Free(id)

	// Internal check (package-level test). If you keep MemAllocator in same package, this is ok.
	if a.rows[id].row != nil {
		t.Fatalf("expected freed slot row to be nil (cleared), got non-nil")
	}
	if a.rows[id].state != slotEmpty {
		t.Fatalf("expected freed slot state=slotEmpty")
	}
}

func TestMemAllocator_MixedWorkload_NoLeakOfAliveState(t *testing.T) {
	a := NewMemAllocator[int]()

	ids := make([]int, 0, 100)

	// Insert 0..49
	for i := 0; i < 50; i++ {
		ids = append(ids, a.Alloc(i))
	}

	// Delete even ids
	for i := 0; i < 50; i += 2 {
		a.Free(ids[i])
	}

	// Reinsert 25 new values; should reuse 25 freed slots
	for i := 0; i < 25; i++ {
		_ = a.Alloc(1000 + i)
	}

	// Verify: all odd original ids still readable; even originals should be either reused with new values or empty
	for i := 0; i < 50; i++ {
		v, ok := a.Get(ids[i])
		if i%2 == 1 {
			if !ok || v != i {
				t.Fatalf("expected odd id=%d to remain alive with value=%d, got v=%d ok=%v", ids[i], i, v, ok)
			}
			continue
		}

		// even ids: could have been reused or still empty depending on allocation order and count.
		// But it must NEVER return the old value i as alive.
		if ok && v == i {
			t.Fatalf("expected even (freed) id=%d not to expose old value=%d", ids[i], i)
		}
	}
}

// BenchmarkMemAllocator_MixedCombined does:
//
//  1. Insert N elements (pre-fill).
//  2. Run a mixed workload for b.N iterations where each iteration randomly does:
//     - Get on a random id
//     - Free on a random id
//     - Alloc a new value (reusing freed slots when possible)
//
// This is a "combined" benchmark intended to stress allocator behavior (free-list reuse),
// branch behavior (alive/empty checks), and cache locality.
//
// Run examples:
//
//	go test -bench=BenchmarkMemAllocator_MixedCombined -benchmem
//
// You can tweak the knobs below or create multiple sub-benchmarks with different N/op mixes.
func BenchmarkMemAllocator_MixedCombined(b *testing.B) {
	// ---- knobs ----
	const (
		N = 1_000_00 // prefill size. change to 1_000, 100_000, 1_000_000 as needed
		// Operation mix (must sum to 100)
		getPct   = 70
		freePct  = 15
		allocPct = 15
		// Value payload is int; swap to []byte if you want to include allocation pressure.
	)
	if getPct+freePct+allocPct != 100 {
		b.Fatalf("op mix must sum to 100, got %d", getPct+freePct+allocPct)
	}

	// Prefill
	a := NewMemAllocator[int]()
	ids := make([]int, N)
	for i := 0; i < N; i++ {
		ids[i] = a.Alloc(i)
	}

	// Pre-generate randomness so the benchmark doesn't measure RNG cost too much.
	// We keep it deterministic for comparability.
	r := rand.New(rand.NewSource(1))
	ops := make([]uint8, b.N)
	picks := make([]int, b.N)
	for i := 0; i < b.N; i++ {
		ops[i] = uint8(r.Intn(100)) // 0..99 => choose operation by percentile
		picks[i] = r.Intn(len(ids)) // random index into ids slice
	}

	// We'll keep inserting new rows. They return internal ids; we store them by replacing
	// the id at a random position (so the pool of "active ids" remains bounded ~N).
	// This prevents ids slice from growing without bound and keeps access patterns stable.
	nextVal := N

	// Reset timer after setup.
	b.ResetTimer()

	// Mixed workload loop
	for i := 0; i < b.N; i++ {
		id := ids[picks[i]]

		switch {
		case ops[i] < getPct:
			_, _ = a.Get(id)

		case ops[i] < getPct+freePct:
			a.Free(id)

		default:
			// Allocate a new row. This will typically reuse freed slots.
			newID := a.Alloc(nextVal)
			nextVal++
			// Replace a random position so subsequent ops target this new row sometimes.
			ids[picks[i]] = newID
		}
	}

	b.StopTimer()

	// Optional: report final allocator state to help interpret results.
	// (This is outside the timed region.)
	_ = a // keep reference
}

// Variant with bytes payload to include allocation/copy/GC pressure.
// Useful if your real rows are blobs.
//
// Run:
//
//	go test -bench=BenchmarkMemAllocator_MixedCombinedBytes -benchmem
func BenchmarkMemAllocator_MixedCombinedBytes(b *testing.B) {
	const (
		N        = 100_000
		payload  = 256 // bytes per row
		getPct   = 70
		freePct  = 15
		allocPct = 15
	)
	if getPct+freePct+allocPct != 100 {
		b.Fatalf("op mix must sum to 100, got %d", getPct+freePct+allocPct)
	}

	a := NewMemAllocator[[]byte]()
	ids := make([]int, N)

	// Prefill: allocate distinct slices to simulate document blobs.
	for i := 0; i < N; i++ {
		buf := make([]byte, payload)
		buf[0] = byte(i) // touch to ensure allocation isn't optimized away
		ids[i] = a.Alloc(buf)
	}

	r := rand.New(rand.NewSource(1))
	ops := make([]uint8, b.N)
	picks := make([]int, b.N)
	for i := 0; i < b.N; i++ {
		ops[i] = uint8(r.Intn(100))
		picks[i] = r.Intn(len(ids))
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		id := ids[picks[i]]

		switch {
		case ops[i] < getPct:
			v, ok := a.Get(id)
			if ok {
				// Touch the slice so compiler can't fully optimize.
				_ = v[0]
			}

		case ops[i] < getPct+freePct:
			a.Free(id)

		default:
			buf := make([]byte, payload)
			buf[0] = 0xAB
			newID := a.Alloc(buf)
			ids[picks[i]] = newID
		}
	}
}
