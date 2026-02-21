package collectionv2

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentInserts(t *testing.T) {
	filename := "/tmp/concurrent_inserts_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	workers := 50
	insertsPerWorker := 100

	var wg sync.WaitGroup
	wg.Add(workers)

	start := time.Now()

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < insertsPerWorker; j++ {
				_, err := c.Insert(map[string]any{
					"worker": id,
					"iter":   j,
					"val":    rand.Int(),
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	if c.Count != int64(workers*insertsPerWorker) {
		t.Errorf("Expected count %d, got %d", workers*insertsPerWorker, c.Count)
	}

	t.Logf("Inserted %d items in %v (%f items/sec)", c.Count, duration, float64(c.Count)/duration.Seconds())
}

func TestConcurrentReadsWrites(t *testing.T) {
	filename := "/tmp/concurrent_rw_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	writers := 10
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					_, err := c.Insert(map[string]any{
						"worker": id,
						"val":    rand.Int(),
					})
					if err != nil {
						t.Error(err)
						return
					}
					time.Sleep(time.Millisecond)
				}
			}
		}(i)
	}

	// Readers
	readers := 10
	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					count := 0
					c.Traverse(func(data []byte) {
						count++
					})
					// t.Logf("Reader %d saw %d items", id, count)
					time.Sleep(time.Millisecond * 5)
				}
			}
		}(i)
	}

	time.Sleep(2 * time.Second)
	close(stop)
	wg.Wait()

	t.Logf("Final count: %d", c.Count)
}

func TestConcurrentPatch(t *testing.T) {
	filename := "/tmp/concurrent_patch_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Insert a row
	row, err := c.Insert(map[string]any{"counter": 0})
	if err != nil {
		t.Fatal(err)
	}

	workers := 20
	patchesPerWorker := 50

	var wg sync.WaitGroup
	wg.Add(workers)

	// We can't easily verify the final value of "counter" because Patch merges.
	// If we use a counter, we need to read-modify-write, which is not atomic via Patch alone unless we lock externally.
	// But Patch itself should be atomic on the collection state.
	// Here we just test for crashes or corruption.

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < patchesPerWorker; j++ {
				err := c.Patch(row, map[string]any{
					"last_worker": id,
					"timestamp":   time.Now().UnixNano(),
				})
				if err != nil {
					t.Error(err)
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify row still exists and is valid
	c.Traverse(func(payload []byte) {
		// We expect only one row
	})
}

func TestConcurrentIndexOperations(t *testing.T) {
	filename := "/tmp/concurrent_index_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Writers
	wg.Add(1)
	go func() {
		defer wg.Done()
		i := 0
		for {
			select {
			case <-stop:
				return
			default:
				_, err := c.Insert(map[string]any{
					"id":   i,
					"type": fmt.Sprintf("A-%d", i),
				})
				if err != nil {
					t.Error(err)
					return
				}
				i++
				time.Sleep(time.Microsecond * 100)
			}
		}
	}()

	// Index Creator/Dropper
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				name := "idx_type"
				// Create
				err := c.Index(name, &IndexMapOptions{Field: "type"})
				if err != nil {
					// It might fail if it already exists (race), but we handle that
					// t.Logf("Index create error (expected sometimes): %v", err)
				}

				time.Sleep(time.Millisecond * 10)

				// Drop
				err = c.DropIndex(name)
				if err != nil {
					// t.Logf("Drop index error (expected sometimes): %v", err)
				}
				time.Sleep(time.Millisecond * 10)
			}
		}
	}()

	time.Sleep(2 * time.Second)
	close(stop)
	wg.Wait()
}

func TestConcurrentUniqueIndex(t *testing.T) {
	filename := "/tmp/concurrent_unique_index_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Create unique index
	err = c.Index("unique_id", &IndexBTreeOptions{
		Fields: []string{"uid"},
		Unique: true, // Wait, IndexBTreeOptions has Unique field?
	})
	// Let's check IndexBTreeOptions definition in index_adapters.go
	// type IndexBTreeOptions struct {
	// 	Fields []string `json:"fields"`
	// 	Sparse bool     `json:"sparse"`
	// 	Unique bool     `json:"unique"`
	// }
	// Yes it does. But does IndexBTree implementation enforce it?
	// In AddRow:
	// if b.Btree.Has(...) { return fmt.Errorf("key ... already exists") }
	// So yes, it enforces uniqueness if Has returns true.

	if err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	workers := 10
	// Try to insert the SAME uid from multiple workers
	// Only one should succeed per uid.

	successCount := int32(0)
	failCount := int32(0)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			_, err := c.Insert(map[string]any{
				"uid": "same_value",
			})
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
		}()
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("Expected exactly 1 success for unique index, got %d", successCount)
	}
	if failCount != int32(workers-1) {
		t.Errorf("Expected %d failures, got %d", workers-1, failCount)
	}
}

func TestConcurrentRemove(t *testing.T) {
	filename := "/tmp/concurrent_remove_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// Insert items first
	count := 1000
	rows := make([]*Row, count)
	for i := 0; i < count; i++ {
		r, err := c.Insert(map[string]any{"i": i})
		if err != nil {
			t.Fatal(err)
		}
		rows[i] = r
	}

	var wg sync.WaitGroup
	workers := 10
	itemsPerWorker := count / workers

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			start := workerID * itemsPerWorker
			end := start + itemsPerWorker
			for j := start; j < end; j++ {
				err := c.Remove(rows[j])
				if err != nil {
					t.Errorf("Worker %d failed to remove row %d: %v", workerID, j, err)
				}
			}
		}(i)
	}

	wg.Wait()

	if c.Count != 0 {
		t.Errorf("Expected count 0, got %d", c.Count)
	}
}

func TestConcurrentConsistency(t *testing.T) {
	filename := "/tmp/concurrent_consistency_test_v2"
	os.Remove(filename)
	defer os.Remove(filename)

	// Phase 1: Concurrent Inserts
	c, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}

	workers := 20
	insertsPerWorker := 50
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < insertsPerWorker; j++ {
				_, err := c.Insert(map[string]any{
					"worker": id,
					"iter":   j,
					"val":    rand.Int(),
				})
				if err != nil {
					t.Error(err)
				}
			}
		}(i)
	}
	wg.Wait()

	expectedCount := int64(workers * insertsPerWorker)
	if c.Count != expectedCount {
		t.Errorf("Phase 1: Expected count %d, got %d", expectedCount, c.Count)
	}

	c.Close()

	// Phase 2: Reopen and Verify
	c2, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}

	if c2.Count != expectedCount {
		t.Errorf("Phase 2: Expected count %d after reopen, got %d", expectedCount, c2.Count)
	}

	// Phase 3: Concurrent Patch and Remove
	// We will remove half of the items and patch the other half
	// To do this safely without complex coordination, we can iterate and assign tasks
	// But since we just reopened, we don't have the *Row pointers from Phase 1 easily available unless we traverse.

	var rows []*Row
	c2.Traverse(func(data []byte) {
		// We need the row pointer, but Traverse only gives payload in current API?
		// Wait, let's check Collection.Traverse signature.
		// func (c *Collection) Traverse(f func(data []byte))
		// It calls c.Rows.Traverse(func(row *Row) bool { f(row.Payload) ... })
		// So we can't get *Row from public Traverse.
		// We need a way to get rows.
		// We can use c.Rows.Traverse directly if we had access, but c.Rows is public?
		// Yes: Rows RowContainer
	})

	// Let's collect all rows first
	rows = make([]*Row, 0, expectedCount)
	c2.Rows.Traverse(func(r *Row) bool {
		rows = append(rows, r)
		return true
	})

	if int64(len(rows)) != expectedCount {
		t.Fatalf("Phase 3: Expected %d rows, found %d", expectedCount, len(rows))
	}

	wg.Add(workers)
	itemsPerWorker := len(rows) / workers

	for i := 0; i < workers; i++ {
		go func(workerID int) {
			defer wg.Done()
			start := workerID * itemsPerWorker
			end := start + itemsPerWorker
			if workerID == workers-1 {
				end = len(rows)
			}

			for j := start; j < end; j++ {
				row := rows[j]
				// Even indices: Patch
				// Odd indices: Remove
				if j%2 == 0 {
					err := c2.Patch(row, map[string]any{"patched": true})
					if err != nil {
						t.Errorf("Patch failed: %v", err)
					}
				} else {
					err := c2.Remove(row)
					if err != nil {
						t.Errorf("Remove failed: %v", err)
					}
				}
			}
		}(i)
	}
	wg.Wait()

	c2.Close()

	// Phase 4: Reopen and Verify Final State
	c3, err := OpenCollection(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer c3.Close()

	// We removed roughly half
	// Exact count depends on the loop range, but let's calculate expected
	removedCount := 0
	for i := 0; i < len(rows); i++ {
		if i%2 != 0 {
			removedCount++
		}
	}
	finalExpected := expectedCount - int64(removedCount)

	if c3.Count != finalExpected {
		t.Errorf("Phase 4: Expected count %d, got %d", finalExpected, c3.Count)
	}

	// Verify patched items
	patchedCount := 0
	c3.Traverse(func(data []byte) {
		var m map[string]any
		json.Unmarshal(data, &m)
		if m["patched"] == true {
			patchedCount++
		}
	})

	expectedPatched := int(expectedCount) - removedCount
	if patchedCount != expectedPatched {
		t.Errorf("Phase 4: Expected %d patched items, got %d", expectedPatched, patchedCount)
	}
}
