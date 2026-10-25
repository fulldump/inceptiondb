package collection

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/valyala/fastjson"
)

// genericMerge applies the patch with the generic (fastjson) merge.
func genericMerge(t *testing.T, doc []byte, plan *PatchPlan) []byte {
	t.Helper()
	var p fastjson.Parser
	v, err := p.ParseBytes(doc)
	if err != nil {
		t.Fatalf("parse %s: %v", doc, err)
	}
	var arena fastjson.Arena
	merged, _ := fastjsonMergePatch(&arena, v, plan.Patch)
	return merged.MarshalTo(nil)
}

func assertSameJSON(t *testing.T, doc, patch string, want, got []byte) {
	t.Helper()
	var a, b any
	if err := json.Unmarshal(want, &a); err != nil {
		t.Fatalf("generic result invalid: %s", want)
	}
	if err := json.Unmarshal(got, &b); err != nil {
		t.Fatalf("raw result is not valid JSON\ndoc:   %s\npatch: %s\ngot:   %s", doc, patch, got)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("raw result differs\ndoc:   %s\npatch: %s\nwant:  %s\ngot:   %s", doc, patch, want, got)
	}
}

func TestRawPatchCases(t *testing.T) {
	cases := []struct{ doc, patch, want string }{
		{`{"a":1,"b":2}`, `{"a":3}`, `{"a":3,"b":2}`},
		{`{"a":1,"b":2}`, `{"c":3}`, `{"a":1,"b":2,"c":3}`},
		{`{}`, `{"c":3}`, `{"c":3}`},
		{` { } `, `{"c":3,"d":"x"}`, ` { "c":3,"d":"x"} `},
		{`{"a":1,"b":2,"c":3}`, `{"a":null}`, `{"b":2,"c":3}`},
		{`{"a":1,"b":2,"c":3}`, `{"b":null}`, `{"a":1,"c":3}`},
		{`{"a":1,"b":2,"c":3}`, `{"c":null}`, `{"a":1,"b":2}`},
		{`{"a":1}`, `{"a":null}`, `{}`},
		{`{"a":1,"b":2}`, `{"a":null,"b":null,"c":true}`, `{"c":true}`},
		{`{ "a" : 1 , "b" : 2 }`, `{"a":null}`, `{ "b" : 2 }`},
		{`{ "a" : 1 , "b" : 2 }`, `{"b":"x"}`, `{ "a" : 1 , "b" : "x" }`},
		{`{"a":{"x":1},"b":[1,2]}`, `{"a":5,"b":[3]}`, `{"a":5,"b":[3]}`},
		{`{"s":"he said \"hi\""}`, `{"s":"a\"b","t":"\u00e9"}`, `{"s":"a\"b","t":"\u00e9"}`},
		{`{"a":1}`, `{"a":1}`, `{"a":1}`},
		{`{"a":1}`, `{"z":null}`, `{"a":1}`},
	}
	for _, c := range cases {
		plan, err := CompilePatch(json.RawMessage(c.patch))
		if err != nil {
			t.Fatal(err)
		}
		if !plan.rawEligible {
			t.Fatalf("patch %s should use the raw path", c.patch)
		}
		got, _, ok := plan.applyRaw([]byte(c.doc))
		if !ok {
			t.Fatalf("raw path refused doc %s", c.doc)
		}
		if string(got) != c.want {
			t.Fatalf("doc %s patch %s: want %s, got %s", c.doc, c.patch, c.want, got)
		}
		assertSameJSON(t, c.doc, c.patch, genericMerge(t, []byte(c.doc), plan), got)
	}
}

func TestRawPatchFallback(t *testing.T) {
	for _, patch := range []string{`{"a":{"b":1}}`, `{"a\"b":1}`, `{"a":1,"a":2}`, `[1]`, `5`} {
		plan, err := CompilePatch(json.RawMessage(patch))
		if err != nil {
			t.Fatal(err)
		}
		if plan.rawEligible {
			t.Fatalf("patch %s must use the generic path", patch)
		}
	}

	// Escaped keys in the document must not be compared raw
	plan, _ := CompilePatch(json.RawMessage(`{"é":1}`))
	if _, _, ok := plan.applyRaw([]byte(`{"\u00e9":0}`)); ok {
		t.Fatal("document with escaped key must use the generic path")
	}
}

func TestRawPatchRandom(t *testing.T) {
	rnd := rand.New(rand.NewSource(1))
	keys := []string{"a", "b", "c", "d", "e"}
	values := []string{`1`, `-2.5e3`, `"x"`, `"q\"uote"`, `true`, `false`, `[]`, `[1,{"n":null}]`, `{"deep":{"x":1}}`, `{}`}
	space := func() string {
		if rnd.Intn(3) == 0 {
			return strings.Repeat(" ", rnd.Intn(3))
		}
		return ""
	}

	for i := 0; i < 20000; i++ {
		// Random document
		parts := []string{}
		for _, k := range rnd.Perm(len(keys))[:rnd.Intn(len(keys)+1)] {
			parts = append(parts, fmt.Sprintf(`%s"%s"%s:%s%s%s`, space(), keys[k], space(), space(), values[rnd.Intn(len(values))], space()))
		}
		doc := space() + "{" + strings.Join(parts, ",") + "}" + space()

		// Random top-level patch (values are never objects)
		patchParts := []string{}
		for _, k := range rnd.Perm(len(keys))[:rnd.Intn(len(keys)+1)] {
			v := values[rnd.Intn(8)]
			if rnd.Intn(3) == 0 {
				v = "null"
			}
			patchParts = append(patchParts, fmt.Sprintf(`"%s":%s`, keys[k], v))
		}
		patch := "{" + strings.Join(patchParts, ",") + "}"

		plan, err := CompilePatch(json.RawMessage(patch))
		if err != nil {
			t.Fatal(err)
		}
		if !plan.rawEligible {
			t.Fatalf("patch %s should use the raw path", patch)
		}
		got, changed, ok := plan.applyRaw([]byte(doc))
		if !ok {
			t.Fatalf("raw path refused doc %s", doc)
		}
		want := genericMerge(t, []byte(doc), plan)
		assertSameJSON(t, doc, patch, want, got)
		if !changed && string(got) != doc {
			t.Fatalf("unchanged result must be the original document")
		}
	}
}

func TestRawPatchFromMap(t *testing.T) {
	plan, err := CompilePatch(map[string]interface{}{"b": 2, "a": "x", "gone": nil})
	if err != nil {
		t.Fatal(err)
	}
	if !plan.rawEligible {
		t.Fatal("map patch should use the raw path")
	}
	got, _, _ := plan.applyRaw([]byte(`{"gone":1,"z":0}`))
	if string(got) != `{"z":0,"a":"x","b":2}` {
		t.Fatalf("got %s", got)
	}
}
