package apicollectionv1

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulldump/inceptiondb/collection"
)

func newTestCollection(t *testing.T) *collection.Collection {

	t.Helper()

	dir := t.TempDir()
	filename := filepath.Join(dir, "collection.jsonl")
	col, err := collection.OpenCollection(filename)
	if err != nil {
		t.Fatalf("open collection: %v", err)
	}

	t.Cleanup(func() {
		col.Drop()
	})

	return col
}

func TestFindRowByID_UsesIndex(t *testing.T) {

	col := newTestCollection(t)

	if err := col.Index("by-id", &collection.IndexMapOptions{Field: "id"}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	if _, err := col.Insert(map[string]any{"id": "doc-1", "name": "Alice"}); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	row, source, err := findRowByID(col, "doc-1")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if row == nil {
		t.Fatalf("expected row, got nil")
	}
	if got := string(row.Payload); !strings.Contains(got, "doc-1") {
		t.Fatalf("unexpected payload: %s", got)
	}
	if source == nil {
		t.Fatalf("expected source metadata")
	}
	if source.Type != "index" || source.Name != "by-id" {
		t.Fatalf("unexpected source: %+v", source)
	}
}

func TestFindRowByID_Fullscan(t *testing.T) {

	col := newTestCollection(t)

	if _, err := col.Insert(map[string]any{"id": "doc-2", "name": "Bob"}); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	row, source, err := findRowByID(col, "doc-2")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if row == nil {
		t.Fatalf("expected row, got nil")
	}
	if got := string(row.Payload); !strings.Contains(got, "doc-2") {
		t.Fatalf("unexpected payload: %s", got)
	}
	if source == nil || source.Type != "fullscan" {
		t.Fatalf("expected fullscan source, got %+v", source)
	}
}

func TestFindRowByID_NotFound(t *testing.T) {

	col := newTestCollection(t)

	if _, err := col.Insert(map[string]any{"id": "doc-3"}); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	row, source, err := findRowByID(col, "missing")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if row != nil {
		t.Fatalf("expected nil row, got %+v", row)
	}
	if source != nil {
		t.Fatalf("expected nil source, got %+v", source)
	}
}
