package apicollectionv1

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"

	"github.com/buger/jsonparser"
)

// referenceMatch is the previous jsonparser based implementation.
func referenceMatch(filter map[string]interface{}, payload []byte) bool {
	for k, expected := range filter {
		val, dataType, _, err := jsonparser.Get(payload, k)
		if err != nil {
			if expected != nil {
				return false
			}
			continue
		}
		switch exp := expected.(type) {
		case string:
			s, err := jsonparser.ParseString(val)
			if dataType != jsonparser.String || err != nil || s != exp {
				return false
			}
		case float64:
			n, err := jsonparser.ParseFloat(val)
			if dataType != jsonparser.Number || err != nil || n != exp {
				return false
			}
		case bool:
			b, err := jsonparser.ParseBoolean(val)
			if dataType != jsonparser.Boolean || err != nil || b != exp {
				return false
			}
		case nil:
			if dataType != jsonparser.Null {
				return false
			}
		}
	}
	return true
}

func TestEqualityFilterMatchesReference(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	values := []string{`5`, `5.0`, `5e0`, `50`, `-5`, `0`, `-0`, `0.5`, `9007199254740993`, `9007199254740992`,
		`"a"`, `"b"`, `"5"`, `"a\"b"`, `"a"`, `true`, `false`, `null`, `[5]`, `{"k":5}`}
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
