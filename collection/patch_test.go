package collection

import (
	"encoding/json"
	"errors"
	"path"
	"testing"

	"github.com/fulldump/inceptiondb/collection/stores"
	"github.com/fulldump/inceptiondb/simdscan"
)

func BenchmarkPatch(b *testing.B) {
	// Setup
	filename := path.Join(b.TempDir(), "bench.wal")
	store, _ := stores.NewStoreDisk(filename)
	col := NewCollection("bench", store)

	payload := []byte(`{"id": 1, "name": "Alice Wonderland", "email": "alice@example.com", "age": 30, "active": true, "balance": 1500.50, "tags": ["premium", "user"], "address": {"city": "Madrid", "zip": "28080"}}`)
	id, _ := col.Insert(payload, false)

	patch := map[string]interface{}{
		"age":     31,
		"balance": 1550.75,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// apply patch
		err := col.Patch(id, patch, false)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestPatchCompiledRawMessage(t *testing.T) {
	filename := path.Join(t.TempDir(), "patch.wal")
	store, err := stores.NewStoreDisk(filename)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	defer store.Close()

	col := NewCollection("patch", store)
	id, err := col.Insert([]byte(`{"name":"Alice","age":30,"active":true}`), true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	plan, err := CompilePatch(json.RawMessage(`{"age":31,"active":true}`))
	if err != nil {
		t.Fatalf("compile patch: %v", err)
	}
	if err := col.Patch(id, plan, true); err != nil {
		t.Fatalf("patch: %v", err)
	}

	data, ok := col.Get(id)
	if !ok {
		t.Fatalf("missing patched document")
	}
	age, _, err := simdscan.GetField(data, "age")
	if err != nil || string(age) != "31" {
		t.Fatalf("age = %q, err = %v, data = %s", age, err, data)
	}

	if err := col.Patch(id, plan, true); err != nil {
		t.Fatalf("second patch: %v", err)
	}
}

func TestPatchRejectsNonObject(t *testing.T) {
	filename := path.Join(t.TempDir(), "patch.wal")
	col, err := OpenCollection(filename)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	original := `{"id":"1","name":"Alice"}`
	id, err := col.Insert([]byte(original), true)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}

	patches := []interface{}{
		json.RawMessage(`"str"`),
		json.RawMessage(` [1,2]`),
		json.RawMessage(`5`),
		json.RawMessage(`true`),
		json.RawMessage(`null`),
		json.RawMessage(``),
		"str",
		[]interface{}{1},
		nil,
	}
	for _, patch := range patches {
		if err := col.Patch(id, patch, true); !errors.Is(err, ErrPatchNotObject) {
			t.Errorf("patch %#v: err = %v, want ErrPatchNotObject", patch, err)
		}
		if _, err := CompilePatch(patch); !errors.Is(err, ErrPatchNotObject) {
			t.Errorf("compile %#v: err = %v, want ErrPatchNotObject", patch, err)
		}
	}

	// A Go value that marshals to an object is still a valid patch
	if err := col.Patch(id, struct {
		Age int `json:"age"`
	}{30}, true); err != nil {
		t.Fatalf("struct patch: %v", err)
	}
	want := `{"id":"1","name":"Alice","age":30}`

	if data, _ := col.Get(id); string(data) != want {
		t.Fatalf("document = %s, want %s", data, want)
	}

	// Nothing reached the journal
	if err := col.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	col, err = OpenCollection(filename)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer col.Close()
	if data, _ := col.Get(id); string(data) != want {
		t.Fatalf("document after reopen = %s, want %s", data, want)
	}
}
