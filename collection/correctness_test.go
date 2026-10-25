package collection

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func openTestCollection(t *testing.T, filename string) *Collection {
	t.Helper()
	col, err := OpenCollection(filename)
	if err != nil {
		t.Fatalf("open collection: %v", err)
	}
	return col
}

func snapshot(col *Collection) map[int64]string {
	out := map[int64]string{}
	col.TraverseRecords(func(id int64, data []byte) bool {
		out[id] = string(data)
		return true
	})
	return out
}

func assertSameSnapshot(t *testing.T, want, got map[int64]string) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("snapshot size: want %d, got %d", len(want), len(got))
	}
	for id, data := range want {
		if got[id] != data {
			t.Fatalf("id %d: want %s, got %s", id, data, got[id])
		}
	}
}

// Recovery must not leave deleted-then-reused ids in the free list, otherwise
// the next insert overwrites a live document.
func TestRecoveredReusedIDIsNotOverwritten(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	const n = 2000

	col := openTestCollection(t, filename)
	ids := make([]int64, 0, n)
	for i := 0; i < n; i++ {
		id, err := col.Insert([]byte(fmt.Sprintf(`{"gen":1,"i":%d}`, i)), false)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := col.Delete(id, false); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < n; i++ {
		if _, err := col.Insert([]byte(fmt.Sprintf(`{"gen":2,"i":%d}`, i)), false); err != nil {
			t.Fatal(err)
		}
	}
	want := snapshot(col)
	if err := col.Close(); err != nil {
		t.Fatal(err)
	}

	col = openTestCollection(t, filename)
	defer col.Close()
	assertSameSnapshot(t, want, snapshot(col))

	for i := 0; i < n; i++ {
		if _, err := col.Insert([]byte(fmt.Sprintf(`{"gen":3,"i":%d}`, i)), false); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(snapshot(col)); got != 2*n {
		t.Fatalf("documents after reinsert: want %d, got %d", 2*n, got)
	}
	if col.Count() != 2*n {
		t.Fatalf("count: want %d, got %d", 2*n, col.Count())
	}
}

// The WAL must replay to exactly the in-memory state, even when operations on
// the same id run concurrently and ids are reused.
func TestConcurrentWritesReplayToSameState(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	const workers = 16
	const rounds = 500
	wg := sync.WaitGroup{}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for r := 0; r < rounds; r++ {
				id, err := col.Insert([]byte(fmt.Sprintf(`{"w":%d,"r":%d}`, w, r)), false)
				if err != nil {
					t.Error(err)
					return
				}
				_ = col.Patch(id, map[string]any{"p": r}, false)
				if r%2 == 0 {
					_ = col.Delete(id, false)
				}
			}
		}(w)
	}
	wg.Wait()

	want := snapshot(col)
	if int64(len(want)) != col.Count() {
		t.Fatalf("count %d does not match documents %d", col.Count(), len(want))
	}
	if err := col.Close(); err != nil {
		t.Fatal(err)
	}

	col = openTestCollection(t, filename)
	defer col.Close()
	assertSameSnapshot(t, want, snapshot(col))
}

// Concurrent patches to the same document must not lose updates, neither in
// memory nor after recovery.
func TestConcurrentPatchSameDocument(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	id, err := col.Insert([]byte(`{}`), false)
	if err != nil {
		t.Fatal(err)
	}

	const workers = 32
	wg := sync.WaitGroup{}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			if err := col.Patch(id, map[string]any{fmt.Sprintf("f%d", w): w}, false); err != nil {
				t.Error(err)
			}
		}(w)
	}
	wg.Wait()

	check := func(col *Collection) {
		data, ok := col.Get(id)
		if !ok {
			t.Fatal("document not found")
		}
		doc := map[string]any{}
		if err := json.Unmarshal(data, &doc); err != nil {
			t.Fatal(err)
		}
		if len(doc) != workers {
			t.Fatalf("lost updates: want %d fields, got %d: %s", workers, len(doc), data)
		}
	}
	check(col)
	if err := col.Close(); err != nil {
		t.Fatal(err)
	}

	col = openTestCollection(t, filename)
	defer col.Close()
	check(col)
}

func TestConcurrentDeleteSameDocument(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	id, _ := col.Insert([]byte(`{"a":1}`), false)
	col.Insert([]byte(`{"a":2}`), false)

	wg := sync.WaitGroup{}
	for w := 0; w < 32; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = col.Delete(id, false)
		}()
	}
	wg.Wait()

	if col.Count() != 1 {
		t.Fatalf("count: want 1, got %d", col.Count())
	}
}

// A patch rejected by a unique index must leave the document and the index
// untouched.
func TestPatchUniqueViolationKeepsState(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	if err := col.CreateIndex("pk", &IndexPKOptions{Paths: [][]string{{"id"}}}); err != nil {
		t.Fatal(err)
	}
	col.Insert([]byte(`{"id":1}`), false)
	id2, _ := col.Insert([]byte(`{"id":2}`), false)

	if err := col.Patch(id2, map[string]any{"id": 1}, false); err == nil {
		t.Fatal("expected duplicate key error")
	}

	data, _ := col.Get(id2)
	if string(data) != `{"id":2}` {
		t.Fatalf("document changed after failed patch: %s", data)
	}
	found := false
	col.TraverseIndex("pk", []byte("2"), func(id int64, data []byte) bool {
		found = id == id2
		return false
	})
	if !found {
		t.Fatal("pk entry lost after failed patch")
	}

	want := snapshot(col)
	col.Close()
	col = openTestCollection(t, filename)
	defer col.Close()
	assertSameSnapshot(t, want, snapshot(col))
}

// A torn record at the end of the WAL (crash or power loss in the middle of
// a write) must not prevent the collection from opening.
func TestTornTailIsTruncated(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)
	for i := 0; i < 10; i++ {
		if _, err := col.Insert([]byte(fmt.Sprintf(`{"i":%d}`, i)), true); err != nil {
			t.Fatal(err)
		}
	}
	col.Close()

	info, err := os.Stat(filename)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(filename, info.Size()-3); err != nil {
		t.Fatal(err)
	}

	col = openTestCollection(t, filename)
	if col.Count() != 9 {
		t.Fatalf("count after torn tail: want 9, got %d", col.Count())
	}
	if _, err := col.Insert([]byte(`{"i":"new"}`), true); err != nil {
		t.Fatal(err)
	}
	col.Close()

	col = openTestCollection(t, filename)
	defer col.Close()
	if col.Count() != 10 {
		t.Fatalf("count after reopen: want 10, got %d", col.Count())
	}
}

// Corruption in the middle of the WAL must be reported, never silently
// dropped together with all the data that follows it.
func TestMidFileCorruptionIsReported(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)
	for i := 0; i < 10; i++ {
		col.Insert([]byte(fmt.Sprintf(`{"i":%d}`, i)), true)
	}
	col.Close()

	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	pos := bytes.Index(data, []byte(`{"i":5}`))
	if pos < 0 {
		t.Fatal("record not found in WAL")
	}
	data[pos+5] = '7' // corrupt a payload byte: the CRC no longer matches
	if err := os.WriteFile(filename, data, 0666); err != nil {
		t.Fatal(err)
	}

	if col, err := OpenCollection(filename); err == nil {
		col.Close()
		t.Fatal("expected corruption error")
	}
}

// Creating an index while inserts are running must index every document.
func TestCreateIndexDuringInserts(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	const workers = 8
	const perWorker = 2000
	wg := sync.WaitGroup{}
	start := make(chan struct{})
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			<-start
			for i := 0; i < perWorker; i++ {
				col.Insert([]byte(fmt.Sprintf(`{"k":"%d-%d"}`, w, i)), false)
			}
		}(w)
	}
	close(start)
	if err := col.CreateIndex("pk", &IndexPKOptions{Paths: [][]string{{"k"}}}); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	missing := 0
	col.TraverseRecords(func(id int64, data []byte) bool {
		k := map[string]string{}
		json.Unmarshal(data, &k)
		found := false
		col.TraverseIndex("pk", []byte(k["k"]), func(got int64, _ []byte) bool {
			found = got == id
			return false
		})
		if !found {
			missing++
		}
		return true
	})
	if missing > 0 {
		t.Fatalf("%d documents missing from index", missing)
	}
}

func TestBtreeKeepsDuplicateKeys(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	if err := col.CreateIndex("age", &IndexBTreeOptions{Fields: []string{"age"}}); err != nil {
		t.Fatal(err)
	}
	ids := []int64{}
	for i := 0; i < 3; i++ {
		id, _ := col.Insert([]byte(`{"age":5}`), false)
		ids = append(ids, id)
	}
	col.Insert([]byte(`{"age":7}`), false)
	col.Delete(ids[1], false)
	col.SyncIndexes()

	count := func(query string) int {
		n := 0
		col.TraverseIndex("age", []byte(query), func(id int64, data []byte) bool {
			n++
			return true
		})
		return n
	}
	if n := count(`{}`); n != 3 {
		t.Fatalf("full scan: want 3, got %d", n)
	}
	if n := count(`{"from":{"age":5},"to":{"age":5}}`); n != 2 {
		t.Fatalf("range age=5: want 2, got %d", n)
	}
	if n := count(`{"reverse":true,"to":{"age":5}}`); n != 2 {
		t.Fatalf("reverse to age=5: want 2, got %d", n)
	}
}

func TestUniqueIndexesConcurrentDuplicates(t *testing.T) {
	cases := map[string]interface{}{
		"pk":    &IndexPKOptions{Paths: [][]string{{"k"}}},
		"map":   &IndexMapOptions{Field: "k"},
		"btree": &IndexBTreeOptions{Fields: []string{"k"}, Unique: true},
	}
	for name, options := range cases {
		t.Run(name, func(t *testing.T) {
			col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
			defer col.Close()
			if err := col.CreateIndex("u", options); err != nil {
				t.Fatal(err)
			}

			wg := sync.WaitGroup{}
			for w := 0; w < 32; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					col.Insert([]byte(`{"k":"same"}`), false)
				}()
			}
			wg.Wait()
			if col.Count() != 1 {
				t.Fatalf("unique key inserted %d times", col.Count())
			}
		})
	}
}

// With strong durability every acknowledged write must be on disk even if
// the process dies without closing the collection.
func TestStrongDurabilitySurvivesCrash(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	spec := DefaultCollectionSpec(filename)
	spec.Durability = DurabilityStrong
	if err := SaveCollectionSpec(spec); err != nil {
		t.Fatal(err)
	}

	col := openTestCollection(t, filename)
	if col.Durability() != DurabilityStrong {
		t.Fatalf("durability: want strong, got %s", col.Durability())
	}

	const workers = 8
	const perWorker = 50
	ids := make(chan int64, workers*perWorker)
	wg := sync.WaitGroup{}
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				id, err := col.Insert([]byte(fmt.Sprintf(`{"w":%d,"i":%d}`, w, i)), false)
				if err != nil {
					t.Error(err)
					return
				}
				ids <- id
			}
		}(w)
	}
	wg.Wait()
	close(ids)

	removed := int64(0)
	for id := range ids {
		if id%3 == 0 {
			if err := col.Delete(id, false); err != nil {
				t.Fatal(err)
			}
			removed++
		} else if err := col.Patch(id, map[string]any{"patched": true}, false); err != nil {
			t.Fatal(err)
		}
	}
	want := snapshot(col)

	// Simulate a crash: reopen the files without closing the collection.
	crashed := openTestCollection(t, filename)
	defer crashed.Close()
	assertSameSnapshot(t, want, snapshot(crashed))
	if crashed.Count() != workers*perWorker-removed {
		t.Fatalf("count: want %d, got %d", workers*perWorker-removed, crashed.Count())
	}
}

func TestDeferredWritesCommit(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	for i := 0; i < 1000; i++ {
		if _, err := col.InsertJSONDeferred([]byte(fmt.Sprintf(`{"i":%d}`, i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := col.Commit(true); err != nil {
		t.Fatal(err)
	}

	crashed := openTestCollection(t, filename)
	defer crashed.Close()
	if crashed.Count() != 1000 {
		t.Fatalf("count after crash: want 1000, got %d", crashed.Count())
	}
}

func TestInvalidDurabilityIsRejected(t *testing.T) {
	spec := DefaultCollectionSpec(filepath.Join(t.TempDir(), "col"))
	spec.Durability = "eventually"
	if col, err := OpenCollectionSpec(spec); err == nil {
		col.Close()
		t.Fatal("expected error")
	}
}

func TestWritesAfterCloseFail(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	id, _ := col.Insert([]byte(`{}`), false)
	col.Close()

	if _, err := col.Insert([]byte(`{}`), false); err == nil {
		t.Fatal("insert after close: expected error")
	}
	if err := col.Patch(id, map[string]any{"a": 1}, false); err == nil {
		t.Fatal("patch after close: expected error")
	}
	if err := col.Delete(id, false); err == nil {
		t.Fatal("delete after close: expected error")
	}
	if err := col.Close(); err != nil {
		t.Fatal(err)
	}
}
