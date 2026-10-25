package collection

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulldump/inceptiondb/collection/stores"
)

func TestIndexMap(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index_map.wal")

	store, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCollection("users", store)

	err = c.Index("by_email", &IndexMapOptions{Field: "email"})
	if err != nil {
		t.Fatal(err)
	}

	mustInsertMap(t, c, map[string]any{"id": 1, "email": "alice@example.com", "name": "Alice"})
	mustInsertMap(t, c, map[string]any{"id": 2, "email": "bob@example.com", "name": "Bob"})
	mustInsertMap(t, c, map[string]any{"id": 3, "email": "charlie@example.com", "name": "Charlie"})
	c.SyncIndexes()

	var found map[string]any
	err = c.TraverseIndex("by_email", mustJSON(t, IndexMapTraverse{Value: "bob@example.com"}), func(id int64, data []byte) bool {
		_ = id
		_ = json.Unmarshal(data, &found)
		return false
	})
	if err != nil {
		t.Fatal(err)
	}

	if found == nil || found["name"] != "Bob" {
		t.Fatalf("expected Bob, got %v", found)
	}

	_, err = c.InsertMap(map[string]any{"id": 4, "email": "alice@example.com", "name": "Alice Duplicate"}, false)
	if err == nil || !strings.Contains(err.Error(), "index conflict") {
		t.Fatalf("expected index conflict, got %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c2 := NewCollection("users", store2)
	if err := c2.Recover(); err != nil {
		t.Fatal(err)
	}
	c2.SyncIndexes()

	found = nil
	err = c2.TraverseIndex("by_email", mustJSON(t, IndexMapTraverse{Value: "alice@example.com"}), func(id int64, data []byte) bool {
		_ = id
		_ = json.Unmarshal(data, &found)
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if found == nil || found["name"] != "Alice" {
		t.Fatalf("expected Alice after reload, got %v", found)
	}

	if err := store2.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIndexBTree(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index_btree.wal")

	store, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCollection("users", store)

	err = c.Index("by_age", &IndexBTreeOptions{Fields: []string{"age"}})
	if err != nil {
		t.Fatal(err)
	}

	mustInsertMap(t, c, map[string]any{"id": 1, "age": 30, "name": "Alice"})
	mustInsertMap(t, c, map[string]any{"id": 2, "age": 20, "name": "Bob"})
	mustInsertMap(t, c, map[string]any{"id": 3, "age": 40, "name": "Charlie"})
	mustInsertMap(t, c, map[string]any{"id": 4, "age": 25, "name": "David"})
	c.SyncIndexes()

	var names []string
	err = c.TraverseIndex("by_age", mustJSON(t, IndexBtreeTraverse{
		From: map[string]interface{}{"age": 20},
		To:   map[string]interface{}{"age": 31},
	}), func(id int64, data []byte) bool {
		_ = id
		var item map[string]any
		_ = json.Unmarshal(data, &item)
		names = append(names, item["name"].(string))
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 3 || names[0] != "Bob" || names[1] != "David" || names[2] != "Alice" {
		t.Fatalf("unexpected order or data: %v", names)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c2 := NewCollection("users", store2)
	if err := c2.Recover(); err != nil {
		t.Fatal(err)
	}
	c2.SyncIndexes()

	names = nil
	err = c2.TraverseIndex("by_age", mustJSON(t, IndexBtreeTraverse{
		From: map[string]interface{}{"age": 25},
		To:   map[string]interface{}{"age": 41},
	}), func(id int64, data []byte) bool {
		_ = id
		var item map[string]any
		_ = json.Unmarshal(data, &item)
		names = append(names, item["name"].(string))
		return true
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(names) != 3 || names[0] != "David" || names[2] != "Charlie" {
		t.Fatalf("unexpected data after reload: %v", names)
	}

	if err := store2.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestIndexBTreeBoundsSymmetryAndCompoundPrefix(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index_btree_bounds.wal")

	store, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	c := NewCollection("users", store)

	err = c.Index("by_age", &IndexBTreeOptions{Fields: []string{"age"}})
	if err != nil {
		t.Fatal(err)
	}

	mustInsertMap(t, c, map[string]any{"id": "1", "age": 30, "name": "Alice"})
	mustInsertMap(t, c, map[string]any{"id": "2", "age": 20, "name": "Bob"})
	mustInsertMap(t, c, map[string]any{"id": "3", "age": 40, "name": "Charlie"})
	mustInsertMap(t, c, map[string]any{"id": "4", "age": 25, "name": "David"})
	c.SyncIndexes()

	queryIDs := func(indexName string, opts IndexBtreeTraverse) []string {
		t.Helper()
		ids := []string{}
		err := c.TraverseIndex(indexName, mustJSON(t, opts), func(id int64, data []byte) bool {
			_ = id
			item := map[string]any{}
			_ = json.Unmarshal(data, &item)
			ids = append(ids, item["id"].(string))
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		return ids
	}

	assertIDs := func(got, expected []string) {
		t.Helper()
		if len(got) != len(expected) {
			t.Fatalf("unexpected length: got=%v expected=%v", got, expected)
		}
		for i := range expected {
			if got[i] != expected[i] {
				t.Fatalf("unexpected ids: got=%v expected=%v", got, expected)
			}
		}
	}

	defaultForward := queryIDs("by_age", IndexBtreeTraverse{
		From: map[string]interface{}{"age": 25},
		To:   map[string]interface{}{"age": 30},
	})
	defaultReverse := queryIDs("by_age", IndexBtreeTraverse{
		From:    map[string]interface{}{"age": 25},
		To:      map[string]interface{}{"age": 30},
		Reverse: true,
	})

	assertIDs(defaultForward, []string{"4", "1"})
	assertIDs(defaultReverse, []string{"1", "4"})

	exclusiveForward := queryIDs("by_age", IndexBtreeTraverse{
		FromExclusive: map[string]interface{}{"age": 20},
		ToExclusive:   map[string]interface{}{"age": 40},
	})
	exclusiveReverse := queryIDs("by_age", IndexBtreeTraverse{
		FromExclusive: map[string]interface{}{"age": 20},
		ToExclusive:   map[string]interface{}{"age": 40},
		Reverse:       true,
	})

	assertIDs(exclusiveForward, []string{"4", "1"})
	assertIDs(exclusiveReverse, []string{"1", "4"})

	err = c.Index("by_category_product", &IndexBTreeOptions{Fields: []string{"category", "-product"}, Sparse: true})
	if err != nil {
		t.Fatal(err)
	}

	mustInsertMap(t, c, map[string]any{"id": "a", "category": "fruit", "product": "orange"})
	mustInsertMap(t, c, map[string]any{"id": "b", "category": "fruit", "product": "apple"})
	mustInsertMap(t, c, map[string]any{"id": "c", "category": "drink", "product": "water"})
	mustInsertMap(t, c, map[string]any{"id": "d", "category": "drink", "product": "milk"})
	c.SyncIndexes()

	prefixForward := queryIDs("by_category_product", IndexBtreeTraverse{
		From: map[string]interface{}{"category": "fruit"},
		To:   map[string]interface{}{"category": "fruit"},
	})
	prefixReverse := queryIDs("by_category_product", IndexBtreeTraverse{
		From:    map[string]interface{}{"category": "fruit"},
		To:      map[string]interface{}{"category": "fruit"},
		Reverse: true,
	})

	assertIDs(prefixForward, []string{"a", "b"})
	assertIDs(prefixReverse, []string{"b", "a"})
}

func TestIndexFTS(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "index_fts.wal")

	store, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c := NewCollection("docs", store)

	err = c.Index("by_content", &IndexFTSOptions{Field: "content"})
	if err != nil {
		t.Fatal(err)
	}

	mustInsertMap(t, c, map[string]any{"id": 1, "content": "hello world"})
	mustInsertMap(t, c, map[string]any{"id": 2, "content": "hello there"})
	mustInsertMap(t, c, map[string]any{"id": 3, "content": "world of go"})
	c.SyncIndexes()

	count := 0
	err = c.TraverseIndex("by_content", mustJSON(t, IndexFTSTraverse{Match: "hello"}), func(id int64, data []byte) bool {
		_ = id
		_ = data
		count++
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("expected 2 rows for hello, got %d", count)
	}

	count = 0
	err = c.TraverseIndex("by_content", mustJSON(t, IndexFTSTraverse{Match: "hello world"}), func(id int64, data []byte) bool {
		_ = id
		_ = data
		count++
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for hello world, got %d", count)
	}

	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store2, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatal(err)
	}
	c2 := NewCollection("docs", store2)
	if err := c2.Recover(); err != nil {
		t.Fatal(err)
	}
	c2.SyncIndexes()

	count = 0
	err = c2.TraverseIndex("by_content", mustJSON(t, IndexFTSTraverse{Match: "go"}), func(id int64, data []byte) bool {
		_ = id
		_ = data
		count++
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 row for go after reload, got %d", count)
	}

	if err := store2.Close(); err != nil {
		t.Fatal(err)
	}
}

func mustJSON(t *testing.T, value interface{}) []byte {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal options: %v", err)
	}
	return b
}

func mustInsertMap(t *testing.T, c *Collection, item map[string]any) int64 {
	t.Helper()
	id, err := c.InsertMap(item, false)
	if err != nil {
		t.Fatalf("insert map: %v", err)
	}
	return id
}
