package apicollectionv1

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/fulldump/inceptiondb/collectionv4"
)

func newTestCollection(t *testing.T) *collectionv4.Collection {

	t.Helper()

	dir := t.TempDir()
	filename := filepath.Join(dir, "collection.jsonl")
	col, err := collectionv4.OpenCollection(filename)
	if err != nil {
		t.Fatalf("open collection: %v", err)
	}

	t.Cleanup(func() {
		//		col.Drop() // TODO: drop collection!
	})

	return col
}

func TestFindRowByID_UsesIndex(t *testing.T) {

	t.SkipNow()

	col := newTestCollection(t)

	if err := col.Index("by-id", &collectionv4.IndexMapOptions{Field: "id"}); err != nil {
		t.Fatalf("create index: %v", err)
	}

	if _, err := col.InsertMap(map[string]any{"id": "doc-1", "name": "Alice"}, false); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	payload, source, err := findRowByID(col, "doc-1")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload, got nil")
	}
	if got := string(payload); !strings.Contains(got, "doc-1") {
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

	if _, err := col.InsertMap(map[string]any{"id": "doc-2", "name": "Bob"}, false); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	payload, source, err := findRowByID(col, "doc-2")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if payload == nil {
		t.Fatalf("expected payload, got nil")
	}
	if got := string(payload); !strings.Contains(got, "doc-2") {
		t.Fatalf("unexpected payload: %s", got)
	}
	if source == nil || source.Type != "fullscan" {
		t.Fatalf("expected fullscan source, got %+v", source)
	}
}

func TestFindRowByID_NotFound(t *testing.T) {

	col := newTestCollection(t)

	if _, err := col.InsertMap(map[string]any{"id": "doc-3"}, false); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	payload, source, err := findRowByID(col, "missing")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if payload != nil {
		t.Fatalf("expected nil payload, got %+v", payload)
	}
	if source != nil {
		t.Fatalf("expected nil source, got %+v", source)
	}
}
