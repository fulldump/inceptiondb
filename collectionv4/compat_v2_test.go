package collectionv4

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/fulldump/inceptiondb/collectionv2"
	"github.com/fulldump/inceptiondb/collectionv4/stores"
)

func TestComparatorCollectionV2VsV4(t *testing.T) {
	baseDir := t.TempDir()

	v2Path := filepath.Join(baseDir, "v2.wal")
	v4Path := filepath.Join(baseDir, "v4.wal")

	v2, err := openV2Harness(v2Path)
	if err != nil {
		t.Fatal(err)
	}
	defer v2.close(t)

	v4, err := openV4Harness(v4Path)
	if err != nil {
		t.Fatal(err)
	}
	defer v4.close(t)

	if err := v2.createIndexes(); err != nil {
		t.Fatal(err)
	}
	if err := v4.createIndexes(); err != nil {
		t.Fatal(err)
	}

	docs := []map[string]any{
		{"id": 1, "email": "alice@example.com", "name": "Alice", "age": 30, "content": "hello world"},
		{"id": 2, "email": "bob@example.com", "name": "Bob", "age": 20, "content": "hello there"},
		{"id": 3, "email": "charlie@example.com", "name": "Charlie", "age": 40, "content": "world of go"},
		{"id": 4, "email": "david@example.com", "name": "David", "age": 25, "content": "golang and databases"},
	}

	for _, doc := range docs {
		if err := v2.insert(doc); err != nil {
			t.Fatal(err)
		}
		if err := v4.insert(doc); err != nil {
			t.Fatal(err)
		}
	}

	v4.col.SyncIndexes()
	assertEqualQueries(t, v2, v4)

	if err := v2.patchByEmail("alice@example.com", map[string]any{"content": "hello cosmos"}); err != nil {
		t.Fatal(err)
	}
	if err := v4.patchByEmail("alice@example.com", map[string]any{"content": "hello cosmos"}); err != nil {
		t.Fatal(err)
	}

	if err := v2.deleteByEmail("bob@example.com"); err != nil {
		t.Fatal(err)
	}
	if err := v4.deleteByEmail("bob@example.com"); err != nil {
		t.Fatal(err)
	}

	v4.col.SyncIndexes()
	assertEqualQueries(t, v2, v4)

	v2BeforeReload := v2.dumpAllCanonical(t)
	v4BeforeReload := v4.dumpAllCanonical(t)
	if !reflect.DeepEqual(v2BeforeReload, v4BeforeReload) {
		t.Fatalf("state mismatch before reload\nv2=%v\nv4=%v", v2BeforeReload, v4BeforeReload)
	}

	if err := v2.reload(); err != nil {
		t.Fatal(err)
	}
	if err := v4.reload(); err != nil {
		t.Fatal(err)
	}

	v4.col.SyncIndexes()
	assertEqualQueries(t, v2, v4)

	v2AfterReload := v2.dumpAllCanonical(t)
	v4AfterReload := v4.dumpAllCanonical(t)
	if !reflect.DeepEqual(v2AfterReload, v4AfterReload) {
		t.Fatalf("state mismatch after reload\nv2=%v\nv4=%v", v2AfterReload, v4AfterReload)
	}
}

func assertEqualQueries(t *testing.T, v2 *v2Harness, v4 *v4Harness) {
	t.Helper()

	v2Map, err := v2.queryMap("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	v4Map, err := v4.queryMap("alice@example.com")
	if err != nil {
		t.Fatal(err)
	}
	assertSameDocuments(t, "map query", v2Map, v4Map, false)

	v2Range, err := v2.queryBTreeRange(20, 41)
	if err != nil {
		t.Fatal(err)
	}
	v4Range, err := v4.queryBTreeRange(20, 41)
	if err != nil {
		t.Fatal(err)
	}
	assertSameDocuments(t, "btree range query", v2Range, v4Range, true)

	v2FTS, err := v2.queryFTS("hello")
	if err != nil {
		t.Fatal(err)
	}
	v4FTS, err := v4.queryFTS("hello")
	if err != nil {
		t.Fatal(err)
	}
	assertSameDocuments(t, "fts query", v2FTS, v4FTS, false)
}

func assertSameDocuments(t *testing.T, label string, left, right []map[string]any, keepOrder bool) {
	t.Helper()

	leftCanonical := canonicalDocs(left)
	rightCanonical := canonicalDocs(right)

	if !keepOrder {
		sort.Strings(leftCanonical)
		sort.Strings(rightCanonical)
	}

	if !reflect.DeepEqual(leftCanonical, rightCanonical) {
		t.Fatalf("%s mismatch\nleft=%v\nright=%v", label, leftCanonical, rightCanonical)
	}
}

func canonicalDocs(docs []map[string]any) []string {
	out := make([]string, 0, len(docs))
	for _, doc := range docs {
		b, _ := json.Marshal(doc)
		out = append(out, string(b))
	}
	return out
}

func decodeDoc(t *testing.T, payload []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	return doc
}

func decodeDocMust(payload []byte) map[string]any {
	var doc map[string]any
	if err := json.Unmarshal(payload, &doc); err != nil {
		panic(err)
	}
	return doc
}

type v2Harness struct {
	path string
	col  *collectionv2.Collection
}

func openV2Harness(path string) (*v2Harness, error) {
	col, err := collectionv2.OpenCollection(path)
	if err != nil {
		return nil, err
	}
	return &v2Harness{path: path, col: col}, nil
}

func (h *v2Harness) close(t *testing.T) {
	t.Helper()
	if h.col != nil {
		if err := h.col.Close(); err != nil {
			t.Fatalf("close v2: %v", err)
		}
		h.col = nil
	}
}

func (h *v2Harness) reload() error {
	if h.col != nil {
		if err := h.col.Close(); err != nil {
			return err
		}
	}
	col, err := collectionv2.OpenCollection(h.path)
	if err != nil {
		return err
	}
	h.col = col
	return nil
}

func (h *v2Harness) createIndexes() error {
	if err := h.col.Index("by_email", &collectionv2.IndexMapOptions{Field: "email"}); err != nil {
		return err
	}
	if err := h.col.Index("by_age", &collectionv2.IndexBTreeOptions{Fields: []string{"age"}}); err != nil {
		return err
	}
	if err := h.col.Index("by_content", &collectionv2.IndexFTSOptions{Field: "content"}); err != nil {
		return err
	}
	return nil
}

func (h *v2Harness) insert(item map[string]any) error {
	_, err := h.col.Insert(item)
	return err
}

func (h *v2Harness) queryMap(value string) ([]map[string]any, error) {
	idx := h.col.Indexes["by_email"]
	opts, err := json.Marshal(collectionv2.IndexMapTraverse{Value: value})
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	idx.Traverse(opts, func(row *collectionv2.Row) bool {
		out = append(out, decodeDocMust(row.Payload))
		return true
	})
	return out, nil
}

func (h *v2Harness) queryBTreeRange(from, to int) ([]map[string]any, error) {
	idx := h.col.Indexes["by_age"]
	opts, err := json.Marshal(collectionv2.IndexBtreeTraverse{
		From: map[string]any{"age": from},
		To:   map[string]any{"age": to},
	})
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	idx.Traverse(opts, func(row *collectionv2.Row) bool {
		out = append(out, decodeDocMust(row.Payload))
		return true
	})
	return out, nil
}

func (h *v2Harness) queryFTS(match string) ([]map[string]any, error) {
	idx := h.col.Indexes["by_content"]
	opts, err := json.Marshal(collectionv2.IndexFTSTraverse{Match: match})
	if err != nil {
		return nil, err
	}

	var out []map[string]any
	idx.Traverse(opts, func(row *collectionv2.Row) bool {
		out = append(out, decodeDocMust(row.Payload))
		return true
	})
	return out, nil
}

func (h *v2Harness) deleteByEmail(email string) error {
	idx := h.col.Indexes["by_email"]
	opts, err := json.Marshal(collectionv2.IndexMapTraverse{Value: email})
	if err != nil {
		return err
	}

	var target *collectionv2.Row
	idx.Traverse(opts, func(row *collectionv2.Row) bool {
		target = row
		return false
	})
	if target == nil {
		return nil
	}
	return h.col.Remove(target)
}

func (h *v2Harness) patchByEmail(email string, patch map[string]any) error {
	idx := h.col.Indexes["by_email"]
	opts, err := json.Marshal(collectionv2.IndexMapTraverse{Value: email})
	if err != nil {
		return err
	}

	var target *collectionv2.Row
	idx.Traverse(opts, func(row *collectionv2.Row) bool {
		target = row
		return false
	})
	if target == nil {
		return nil
	}

	return h.col.Patch(target, patch)
}

func (h *v2Harness) dumpAllCanonical(t *testing.T) []string {
	t.Helper()
	var docs []map[string]any
	h.col.Traverse(func(data []byte) {
		docs = append(docs, decodeDoc(t, data))
	})
	canonical := canonicalDocs(docs)
	sort.Strings(canonical)
	return canonical
}

type v4Harness struct {
	path  string
	store *stores.StoreDisk
	col   *Collection
}

func openV4Harness(path string) (*v4Harness, error) {
	store, err := stores.NewStoreDisk(path)
	if err != nil {
		return nil, err
	}
	return &v4Harness{path: path, store: store, col: NewCollection("cmp", store)}, nil
}

func (h *v4Harness) close(t *testing.T) {
	t.Helper()
	if h.store != nil {
		if err := h.store.Close(); err != nil {
			t.Fatalf("close v4 store: %v", err)
		}
		h.store = nil
		h.col = nil
	}
}

func (h *v4Harness) reload() error {
	if h.store != nil {
		if err := h.store.Close(); err != nil {
			return err
		}
	}
	store, err := stores.NewStoreDisk(h.path)
	if err != nil {
		return err
	}
	col := NewCollection("cmp", store)
	if err := col.Recover(); err != nil {
		_ = store.Close()
		return err
	}
	h.store = store
	h.col = col
	return nil
}

func (h *v4Harness) createIndexes() error {
	if err := h.col.Index("by_email", &IndexMapOptions{Field: "email"}); err != nil {
		return err
	}
	if err := h.col.Index("by_age", &IndexBTreeOptions{Fields: []string{"age"}}); err != nil {
		return err
	}
	if err := h.col.Index("by_content", &IndexFTSOptions{Field: "content"}); err != nil {
		return err
	}
	return nil
}

func (h *v4Harness) insert(item map[string]any) error {
	_, err := h.col.InsertMap(item, false)
	return err
}

func (h *v4Harness) queryMap(value string) ([]map[string]any, error) {
	opts, err := json.Marshal(IndexMapTraverse{Value: value})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	err = h.col.TraverseIndex("by_email", opts, func(id int64, data []byte) bool {
		_ = id
		out = append(out, decodeDocMust(data))
		return true
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *v4Harness) queryBTreeRange(from, to int) ([]map[string]any, error) {
	opts, err := json.Marshal(IndexBtreeTraverse{
		From: map[string]any{"age": from},
		To:   map[string]any{"age": to},
	})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	err = h.col.TraverseIndex("by_age", opts, func(id int64, data []byte) bool {
		_ = id
		out = append(out, decodeDocMust(data))
		return true
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *v4Harness) queryFTS(match string) ([]map[string]any, error) {
	opts, err := json.Marshal(IndexFTSTraverse{Match: match})
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	err = h.col.TraverseIndex("by_content", opts, func(id int64, data []byte) bool {
		_ = id
		out = append(out, decodeDocMust(data))
		return true
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (h *v4Harness) deleteByEmail(email string) error {
	opts, err := json.Marshal(IndexMapTraverse{Value: email})
	if err != nil {
		return err
	}
	var id int64
	found := false
	_ = h.col.TraverseIndex("by_email", opts, func(foundID int64, data []byte) bool {
		_ = data
		id = foundID
		found = true
		return false
	})
	if !found {
		return nil
	}
	return h.col.Delete(id, false)
}

func (h *v4Harness) patchByEmail(email string, patch map[string]any) error {
	opts, err := json.Marshal(IndexMapTraverse{Value: email})
	if err != nil {
		return err
	}
	var id int64 = -1
	err = h.col.TraverseIndex("by_email", opts, func(foundID int64, data []byte) bool {
		_ = data
		id = foundID
		return false
	})
	if err != nil {
		return err
	}
	if id < 0 {
		return nil
	}
	return h.col.Patch(id, patch, false)
}

func (h *v4Harness) dumpAllCanonical(t *testing.T) []string {
	t.Helper()
	var docs []map[string]any
	h.col.Traverse(func(data []byte) {
		docs = append(docs, decodeDoc(t, data))
	})
	canonical := canonicalDocs(docs)
	sort.Strings(canonical)
	return canonical
}
