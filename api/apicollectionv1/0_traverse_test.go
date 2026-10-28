package apicollectionv1

import (
	"testing"
)

// A filter finds the same documents however it is written: the plain
// equality form runs on the fast path, the operator form on connor.
func TestTraverseEqualityLooksInsideArrays(t *testing.T) {

	col := newTestCollection(t)
	for _, doc := range []string{
		`{"id":"1","tags":["vip","beta"]}`,
		`{"id":"2","tags":["beta"]}`,
		`{"id":"3","tags":"vip"}`,
		`{"id":"4","tags":[["vip"]]}`,
		`{"id":"5"}`,
	} {
		if _, err := col.Insert([]byte(doc), false); err != nil {
			t.Fatalf("insert %s: %v", doc, err)
		}
	}

	find := func(body string) []string {
		t.Helper()
		ids := []string{}
		err := traverse([]byte(body), col, func(id int64, payload []byte) bool {
			ids = append(ids, string(payload))
			return true
		})
		if err != nil {
			t.Fatalf("traverse %s: %v", body, err)
		}
		return ids
	}

	plain := find(`{"filter":{"tags":"vip"},"limit":-1}`)
	operator := find(`{"filter":{"tags":{"$eq":"vip"}},"limit":-1}`)
	if len(plain) != 3 || len(operator) != 3 {
		t.Fatalf("documents with vip: plain %d %v, $eq %d %v; want 3 each", len(plain), plain, len(operator), operator)
	}
}
