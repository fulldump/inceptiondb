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
