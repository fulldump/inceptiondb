package collection

import (
	"errors"
	"fmt"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/fulldump/inceptiondb/simdscan"
)

func TestDefaultsFillNullFields(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	if err := col.SetDefaults(map[string]any{"id": "uuid()", "n": 5.0, "c": "auto()"}); err != nil {
		t.Fatalf("set defaults: %v", err)
	}

	field := func(data []byte, key string) (string, simdscan.Type) {
		val, typ, err := simdscan.GetField(data, key)
		if err != nil {
			t.Fatalf("field %s of %s: %v", key, data, err)
		}
		return string(val), typ
	}

	insertJSON := func(doc string) []byte {
		id, err := col.InsertJSON([]byte(doc), false)
		if err != nil {
			t.Fatalf("insert %s: %v", doc, err)
		}
		data, _ := col.Get(id)
		return data
	}
	insertMap := func(doc map[string]any) []byte {
		id, err := col.InsertMap(doc, false)
		if err != nil {
			t.Fatalf("insert %v: %v", doc, err)
		}
		data, _ := col.Get(id)
		return data
	}

	// null counts as not set: the default is applied
	for _, data := range [][]byte{
		insertJSON(`{"id":null,"n":null,"c":null}`),
		insertMap(map[string]any{"id": nil, "n": nil, "c": nil}),
	} {
		if _, typ := field(data, "id"); typ != simdscan.TypeString {
			t.Errorf("id of %s: want a generated uuid", data)
		}
		if n, _ := field(data, "n"); n != "5" {
			t.Errorf("n of %s: want 5", data)
		}
		if _, typ := field(data, "c"); typ != simdscan.TypeNumber {
			t.Errorf("c of %s: want an auto() number", data)
		}
	}

	// Any other value is kept, also false, 0 and ""
	data := insertJSON(`{"id":"mine","n":0,"c":""}`)
	if string(data) != `{"id":"mine","n":0,"c":""}` {
		t.Errorf("document = %s, want it unchanged", data)
	}
}

func TestInsertJSONRejectsNonObject(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	for _, defaults := range []map[string]any{nil, {"id": "uuid()"}} {
		if err := col.SetDefaults(defaults); err != nil {
			t.Fatalf("set defaults: %v", err)
		}
		for _, doc := range []string{`[1]`, `"x"`, `5`, `null`, `true`, ` [1]`, ``} {
			if _, err := col.InsertJSON([]byte(doc), false); !errors.Is(err, ErrDocumentNotObject) {
				t.Errorf("defaults %v, insert %q: err = %v, want ErrDocumentNotObject", defaults, doc, err)
			}
		}
	}
	if n := col.Count(); n != 0 {
		t.Fatalf("count = %d, want 0", n)
	}
}

func autoValue(t *testing.T, col *Collection, id int64, field string) int64 {
	t.Helper()
	data, _ := col.Get(id)
	val, _, err := simdscan.GetField(data, field)
	if err != nil {
		t.Fatalf("field %s of %s: %v", field, data, err)
	}
	var n int64
	fmt.Sscan(string(val), &n)
	return n
}

func TestAutoKeepsGrowingAfterRestart(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	if err := col.SetDefaults(map[string]any{"n": "auto()"}); err != nil {
		t.Fatalf("set defaults: %v", err)
	}
	var ids []int64
	for i := 0; i < 5; i++ {
		id, err := col.InsertJSON([]byte(`{}`), false)
		if err != nil {
			t.Fatalf("insert: %v", err)
		}
		ids = append(ids, id)
	}
	last := autoValue(t, col, ids[4], "n")

	// Count() falls below the largest auto() value
	for _, id := range ids[:4] {
		if err := col.Delete(id, false); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}

	if err := col.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	col = openTestCollection(t, filename)
	defer col.Close()

	id, err := col.InsertJSON([]byte(`{}`), false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := autoValue(t, col, id, "n"); got <= last {
		t.Fatalf("auto() after restart = %d, want > %d", got, last)
	}
}

func TestAutoStartsAfterExistingValues(t *testing.T) {
	col := openTestCollection(t, filepath.Join(t.TempDir(), "col"))
	defer col.Close()

	if _, err := col.InsertJSON([]byte(`{"n":100}`), false); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := col.SetDefaults(map[string]any{"n": "auto()"}); err != nil {
		t.Fatalf("set defaults: %v", err)
	}
	id, err := col.InsertJSON([]byte(`{}`), false)
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	if got := autoValue(t, col, id, "n"); got <= 100 {
		t.Fatalf("auto() = %d, want > 100", got)
	}
}

// TestSizeCountersMatchData checks that the O(1) size counters are kept
// exactly in sync by inserts, patches, deletes and replay.
func TestSizeCountersMatchData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "col")
	col := openTestCollection(t, filename)

	options := map[string]interface{}{
		"map":   &IndexMapOptions{Field: "id"},
		"btree": &IndexBTreeOptions{Fields: []string{"x", "id"}},
		// unique: updated in place, so a patch that keeps k replaces its entry
		"ubtree": &IndexBTreeOptions{Fields: []string{"k"}, Unique: true},
		"pk":     &IndexPKOptions{Paths: [][]string{{"k"}}},
		"fts":    &IndexFTSOptions{Field: "text"},
	}
	for name, opts := range options {
		if err := col.Index(name, opts); err != nil {
			t.Fatalf("create index %s: %v", name, err)
		}
	}

	// Sizes computed from scratch from the current documents
	check := func(when string) {
		t.Helper()
		col.SyncIndexes()

		var data int64
		fresh := map[string]Index{
			"map":    NewIndexMap(options["map"].(*IndexMapOptions)),
			"btree":  NewIndexBTree(options["btree"].(*IndexBTreeOptions)),
			"ubtree": NewIndexBTree(options["ubtree"].(*IndexBTreeOptions)),
			"pk":     NewIndexPK(options["pk"].(*IndexPKOptions)),
			"fts":    NewIndexFTS(options["fts"].(*IndexFTSOptions)),
		}
		col.TraverseRecords(func(id int64, payload []byte) bool {
			data += int64(len(payload))
			for _, index := range fresh {
				if err := index.Add(id, payload); err != nil {
					t.Fatalf("%s: fresh index: %v", when, err)
				}
			}
			return true
		})

		if got := col.DataSize(); got != data {
			t.Errorf("%s: DataSize = %d, want %d", when, got, data)
		}
		if got, want := col.MemorySize(), data+col.Count()*recordOverhead; got != want {
			t.Errorf("%s: MemorySize = %d, want %d", when, got, want)
		}
		for name, index := range col.ListIndexes() {
			if got, want := index.Size(), fresh[name].Size(); got != want {
				t.Errorf("%s: index %s Size = %d, want %d", when, name, got, want)
			}
		}
	}

	words := []string{"red", "green", "blue", "big", "small"}
	doc := func(n int, rnd *rand.Rand) string {
		return fmt.Sprintf(`{"id":"id-%d","k":"key-%d","x":%d,"text":"%s %s"}`,
			n, n, rnd.Intn(10), words[rnd.Intn(len(words))], words[rnd.Intn(len(words))])
	}

	rnd := rand.New(rand.NewSource(1))
	var live []int64
	for i := 0; i < 2000; i++ {
		switch op := rnd.Intn(10); {
		case op < 5 || len(live) == 0:
			id, err := col.Insert([]byte(doc(i, rnd)), false)
			if err != nil {
				t.Fatalf("insert: %v", err)
			}
			live = append(live, id)
		case op < 8:
			id := live[rnd.Intn(len(live))]
			patch := map[string]any{"x": rnd.Intn(10), "text": words[rnd.Intn(len(words))]}
			if rnd.Intn(4) == 0 {
				patch["extra"] = "some longer value"
			}
			if err := col.Patch(id, patch, false); err != nil {
				t.Fatalf("patch: %v", err)
			}
		default:
			j := rnd.Intn(len(live))
			if err := col.Delete(live[j], false); err != nil {
				t.Fatalf("delete: %v", err)
			}
			live = append(live[:j], live[j+1:]...)
		}
	}
	check("after writes")

	if err := col.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	col = openTestCollection(t, filename)
	defer col.Close()
	check("after reopen")

	for _, id := range live {
		if err := col.Delete(id, false); err != nil {
			t.Fatalf("delete: %v", err)
		}
	}
	check("after deleting everything")
	if got := col.DataSize(); got != 0 {
		t.Errorf("DataSize = %d, want 0", got)
	}
}
