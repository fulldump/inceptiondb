package simdscan

import "testing"

func TestEachField(t *testing.T) {
	data := []byte(` { "a" : 1 , "b":"x\"y", "c":[1,{"d":2}], "e":{"f":null}, "g":true, "h":false, "i":null } `)
	want := []struct {
		key, value string
		typ        Type
	}{
		{"a", `1`, TypeNumber},
		{"b", `"x\"y"`, TypeString},
		{"c", `[1,{"d":2}]`, TypeArray},
		{"e", `{"f":null}`, TypeObject},
		{"g", `true`, TypeBoolean},
		{"h", `false`, TypeBoolean},
		{"i", `null`, TypeNull},
	}
	i := 0
	end, err := EachField(data, func(f Field) bool {
		if i >= len(want) {
			t.Fatalf("unexpected field %s", f.Key)
		}
		w := want[i]
		if string(f.Key) != w.key || string(data[f.ValueStart:f.ValueEnd]) != w.value || f.Type != w.typ || data[f.KeyStart] != '"' {
			t.Fatalf("field %d: got key %s value %s type %v", i, f.Key, data[f.ValueStart:f.ValueEnd], f.Type)
		}
		i++
		return true
	})
	if err != nil || i != len(want) || data[end] != '}' {
		t.Fatalf("end=%d err=%v fields=%d", end, err, i)
	}
}

func TestEachFieldBadJSON(t *testing.T) {
	for _, data := range []string{``, `[]`, `{`, `{"a"}`, `{"a":}`, `{"a":1`, `{"a":1 "b":2}`, `{"a":tru}`, `{"a":"x}`, `{a:1}`} {
		if _, err := EachField([]byte(data), func(Field) bool { return true }); err == nil {
			t.Fatalf("%q: expected error", data)
		}
	}
}
