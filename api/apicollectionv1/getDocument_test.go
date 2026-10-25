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
		//		col.Drop() // TODO: drop collection!
	})

	return col
}

func TestFindRowByID_UsesIndex(t *testing.T) {

	col := newTestCollection(t)

	if err := col.Index("by-id", &collection.IndexMapOptions{Field: "id"}); err != nil {
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

func TestFindRowByID_UsesPKIndex(t *testing.T) {

	col := newTestCollection(t)

	if err := col.Index("pk", &collection.IndexPKOptions{Paths: [][]string{{"id"}}}); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if _, err := col.InsertMap(map[string]any{"id": "doc-4"}, false); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	payload, source, err := findRowByID(col, "doc-4")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if !strings.Contains(string(payload), "doc-4") {
		t.Fatalf("unexpected payload: %s", payload)
	}
	if source == nil || source.Type != "index" || source.Name != "pk" {
		t.Fatalf("unexpected source: %+v", source)
	}

	payload, source, err = findRowByID(col, "missing")
	if err != nil || payload != nil || source != nil {
		t.Fatalf("missing document: payload %s, source %+v, err %v", payload, source, err)
	}
}

func TestFindRowByID_OtherIndexUsesFullscan(t *testing.T) {

	col := newTestCollection(t)

	if err := col.Index("by-name", &collection.IndexMapOptions{Field: "name"}); err != nil {
		t.Fatalf("create index: %v", err)
	}
	if _, err := col.Insert([]byte(`{"id":7,"name":"x"}`), false); err != nil {
		t.Fatalf("insert document: %v", err)
	}

	payload, source, err := findRowByID(col, "7")
	if err != nil {
		t.Fatalf("findRowByID: %v", err)
	}
	if string(payload) != `{"id":7,"name":"x"}` {
		t.Fatalf("unexpected payload: %s", payload)
	}
	if source == nil || source.Type != "fullscan" {
		t.Fatalf("expected fullscan source, got %+v", source)
	}
}
