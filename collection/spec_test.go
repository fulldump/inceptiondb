package collection

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/fulldump/inceptiondb/collection/records"
)

func TestOpenCollectionSpecUsesConfiguredRecordsOnRecover(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "users.wal")
	spec := DefaultCollectionSpec(filename)
	spec.Store.Wrappers = nil
	spec.Records.Engine = RecordsFast

	col, err := OpenCollectionSpec(spec)
	if err != nil {
		t.Fatalf("open collection: %v", err)
	}
	defer col.Close()

	if _, err := col.Insert([]byte(`{"name":"Alice"}`), true); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := col.Recover(); err != nil {
		t.Fatalf("recover: %v", err)
	}

	want := reflect.TypeOf(records.NewRecordsFast[Record]())
	got := reflect.TypeOf(col.records)
	if got != want {
		t.Fatalf("records engine = %v, want %v", got, want)
	}
}

func TestOpenCollectionSpecRejectsUnknownComponents(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "users.wal")

	spec := DefaultCollectionSpec(filename)
	spec.Store.Backend = "missing"
	if _, err := OpenCollectionSpec(spec); err == nil {
		t.Fatalf("expected unknown backend error")
	}

	spec = DefaultCollectionSpec(filename)
	spec.Store.Wrappers = []StoreWrapperSpec{{Type: "missing"}}
	if _, err := OpenCollectionSpec(spec); err == nil {
		t.Fatalf("expected unknown wrapper error")
	}

	spec = DefaultCollectionSpec(filename)
	spec.Records.Engine = "missing"
	if _, err := OpenCollectionSpec(spec); err == nil {
		t.Fatalf("expected unknown records error")
	}
}

func TestSaveLoadCollectionSpec(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "events.wal")
	spec := DefaultCollectionSpec(filename)
	spec.Name = "events"
	spec.Store = StoreSpec{
		Backend: StoreBackendDisk,
		Wrappers: []StoreWrapperSpec{
			{Type: StoreWrapperSnappy},
			{Type: StoreWrapperAsync},
		},
	}
	spec.Records.Engine = RecordsTurbo

	if err := SaveCollectionSpec(spec); err != nil {
		t.Fatalf("save spec: %v", err)
	}

	loaded, ok, err := LoadCollectionSpec(filename)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if !ok {
		t.Fatalf("expected persisted spec")
	}
	if loaded.Filename != filename {
		t.Fatalf("filename = %q, want %q", loaded.Filename, filename)
	}
	if loaded.Name != spec.Name || loaded.Store.Backend != spec.Store.Backend || loaded.Records.Engine != spec.Records.Engine {
		t.Fatalf("loaded spec = %#v, want %#v", loaded, spec)
	}
	if len(loaded.Store.Wrappers) != 2 || loaded.Store.Wrappers[0].Type != StoreWrapperSnappy || loaded.Store.Wrappers[1].Type != StoreWrapperAsync {
		t.Fatalf("wrappers = %#v", loaded.Store.Wrappers)
	}
}
