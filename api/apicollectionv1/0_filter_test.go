package apicollectionv1

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/SierraSoftworks/connor"
	"github.com/buger/jsonparser"
)

// referenceMatch is the previous jsonparser based implementation, with the
// array semantics of connor: an array matches when any element does.
func referenceMatch(filter map[string]interface{}, payload []byte) bool {
	for k, expected := range filter {
		val, dataType, _, err := jsonparser.Get(payload, k)
		if err != nil {
			if expected != nil {
				return false
			}
			continue
		}
		if !referenceEqual(expected, val, dataType) {
			return false
		}
	}
	return true
}

func referenceEqual(expected interface{}, val []byte, dataType jsonparser.ValueType) bool {
	if dataType == jsonparser.Array {
		found := false
		jsonparser.ArrayEach(val, func(v []byte, t jsonparser.ValueType, _ int, _ error) {
			found = found || referenceEqual(expected, v, t)
		})
		return found
	}
	switch exp := expected.(type) {
	case string:
		s, err := jsonparser.ParseString(val)
		return dataType == jsonparser.String && err == nil && s == exp
	case float64:
		n, err := jsonparser.ParseFloat(val)
		return dataType == jsonparser.Number && err == nil && n == exp
	case bool:
		b, err := jsonparser.ParseBoolean(val)
		return dataType == jsonparser.Boolean && err == nil && b == exp
	case nil:
		return dataType == jsonparser.Null
	}
	return false
}

func TestEqualityFilterMatchesReference(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	values := []string{`5`, `5.0`, `5e0`, `50`, `-5`, `0`, `-0`, `0.5`, `9007199254740993`, `9007199254740992`,
		`"a"`, `"b"`, `"5"`, `"a\"b"`, `"a"`, `true`, `false`, `null`, `[5]`, `{"k":5}`,
		`[]`, `["a","b"]`, `["b",5]`, `[["a"]]`, `[null]`, `[true,false]`, `[{"k":"a"}]`, `["a\"b"]`, `[0.5, -5]`}
	filters := []string{`5`, `-5`, `0`, `0.5`, `9007199254740992`, `"a"`, `"a\"b"`, `"5"`, `true`, `false`, `null`}
	keys := []string{"k", "x"}

	for i := 0; i < 20000; i++ {
		parts := []string{}
		for _, k := range keys {
			if rnd.Intn(4) > 0 {
				parts = append(parts, fmt.Sprintf(`"%s":%s`, k, values[rnd.Intn(len(values))]))
			}
		}
		doc := []byte("{" + strings.Join(parts, ",") + "}")

		filterJSON := "{"
		for j, k := range keys[:1+rnd.Intn(len(keys))] {
			if j > 0 {
				filterJSON += ","
			}
			filterJSON += fmt.Sprintf(`"%s":%s`, k, filters[rnd.Intn(len(filters))])
		}
		filterJSON += "}"
		filter := map[string]interface{}{}
		if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
			t.Fatal(err)
		}

		want := referenceMatch(filter, doc)
		if got := compileEqualityFilter(filter).match(doc); got != want {
			t.Fatalf("doc %s filter %s: want %v, got %v", doc, filterJSON, want, got)
		}
	}
}

// TestEqualityFilterMatchesConnor checks the fast path against connor, the
// matcher used for any other filter: a filter must not change its results
// with the way it is written ({"tags": "a"} and {"tags": {"$eq": "a"}}).
func TestEqualityFilterMatchesConnor(t *testing.T) {
	docs := []string{
		`{"tags":["a","b"]}`, `{"tags":"a"}`, `{"tags":[]}`, `{"tags":[["a"]]}`, `{"tags":["b",5]}`,
		`{"tags":[null]}`, `{"tags":null}`, `{}`, `{"tags":[true]}`, `{"tags":[{"k":"a"}]}`, `{"tags":5}`, `{"tags":[5.5]}`,
	}
	filters := []string{`{"tags":"a"}`, `{"tags":5}`, `{"tags":5.5}`, `{"tags":true}`, `{"tags":null}`, `{"tags":"b"}`}
	for _, d := range docs {
		for _, f := range filters {
			filter := map[string]interface{}{}
			doc := map[string]interface{}{}
			json.Unmarshal([]byte(f), &filter)
			json.Unmarshal([]byte(d), &doc)
			want, err := connor.Match(filter, doc)
			if err != nil {
				t.Fatal(err)
			}
			if got := compileEqualityFilter(filter).match([]byte(d)); got != want {
				t.Errorf("doc %s filter %s: connor %v, fast path %v", d, f, want, got)
			}
		}
	}
}

func BenchmarkEqualityFilter(b *testing.B) {
	doc := []byte(`{"id":"123456","value":0,"worker":5}`)
	filter := map[string]interface{}{"worker": float64(7)}
	b.Run("reference", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			referenceMatch(filter, doc)
		}
	})
	compiled := compileEqualityFilter(filter)
	b.Run("compiled", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			compiled.match(doc)
		}
	})
}
