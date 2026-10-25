package collection

import (
	"bytes"
	"encoding/json"
	"sort"

	"github.com/fulldump/inceptiondb/simdscan"
)

// rawPatchMaxFields bounds the fields of a patch eligible for the raw path
// (tracked with a 64 bit mask).
const rawPatchMaxFields = 64

// rawPatchField is a top-level field of a merge patch whose value replaces
// (or deletes) the value in the document, so no recursive merge is needed.
type rawPatchField struct {
	key    []byte // as written in JSON, without quotes; contains no escapes
	value  []byte // raw JSON value
	delete bool
}

// compileRaw prepares the raw path: a patch of top-level fields whose values
// are not objects is applied by rewriting byte ranges of the document instead
// of parsing, merging and serializing it. Other patches use the generic merge.
func (p *PatchPlan) compileRaw(patch interface{}) {
	var fields []rawPatchField

	switch v := patch.(type) {
	case json.RawMessage:
		eligible := true
		_, err := simdscan.EachField(v, func(f simdscan.Field) bool {
			if f.Type == simdscan.TypeObject || bytes.IndexByte(f.Key, '\\') >= 0 {
				eligible = false
				return false
			}
			fields = append(fields, rawPatchField{
				key:    f.Key,
				value:  v[f.ValueStart:f.ValueEnd],
				delete: f.Type == simdscan.TypeNull,
			})
			return true
		})
		if err != nil || !eligible {
			return
		}

	case map[string]interface{}:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys) // deterministic order for appended fields
		for _, k := range keys {
			quoted, err := json.Marshal(k)
			if err != nil || !bytes.Equal(quoted[1:len(quoted)-1], []byte(k)) {
				return // the key needs escaping
			}
			if v[k] == nil {
				fields = append(fields, rawPatchField{key: []byte(k), delete: true})
				continue
			}
			if _, isObject := v[k].(map[string]interface{}); isObject {
				return
			}
			value, err := json.Marshal(v[k])
			if err != nil || (len(value) > 0 && value[0] == '{') {
				return
			}
			fields = append(fields, rawPatchField{key: []byte(k), value: value})
		}

	default:
		return
	}

	if len(fields) > rawPatchMaxFields {
		return
	}
	for i := range fields {
		for j := 0; j < i; j++ {
			if bytes.Equal(fields[i].key, fields[j].key) {
				return // duplicated keys: let the generic path decide
			}
		}
	}

	p.raw = fields
	p.rawEligible = true
	for _, f := range fields {
		if !f.delete {
			p.rawExtra += len(f.key) + len(f.value) + 4 // ,"key":value
		}
	}
}

// applyRaw applies the patch to doc by copying it and replacing the byte
// ranges of the patched values. Untouched bytes are kept as they are. It
// returns ok=false when the document needs the generic path.
func (p *PatchPlan) applyRaw(doc []byte) (out []byte, changed bool, ok bool) {
	last := 0          // doc[:last] has been handled
	emitted := false   // a field of doc has been kept in out
	skipComma := false // leading fields were deleted: drop the separator before the next kept one
	prevEnd := 0       // end of the previous field value
	var seen uint64
	ok = true

	replace := func(from, to int, with []byte) {
		if out == nil {
			out = make([]byte, 0, len(doc)+p.rawExtra)
		}
		out = append(out, doc[last:from]...)
		out = append(out, with...)
		last = to
	}

	end, err := simdscan.EachField(doc, func(f simdscan.Field) bool {
		if bytes.IndexByte(f.Key, '\\') >= 0 {
			ok = false // escaped key: raw comparison is not reliable
			return false
		}

		match := -1
		for j := range p.raw {
			if bytes.Equal(p.raw[j].key, f.Key) {
				match = j
				seen |= 1 << j
				break
			}
		}

		if match >= 0 && p.raw[match].delete {
			switch {
			case emitted:
				replace(prevEnd, f.ValueEnd, nil) // with its leading comma
			case skipComma:
				last = f.ValueEnd // keep dropping leading fields and their separators
			default:
				replace(f.KeyStart, f.ValueEnd, nil)
				skipComma = true
			}
			prevEnd = f.ValueEnd
			return true
		}

		if skipComma {
			last = f.KeyStart // drop the separator after the deleted fields
			skipComma = false
		}
		if match >= 0 && !bytes.Equal(doc[f.ValueStart:f.ValueEnd], p.raw[match].value) {
			replace(f.ValueStart, f.ValueEnd, p.raw[match].value)
		}
		emitted = true
		prevEnd = f.ValueEnd
		return true
	})
	if err != nil || !ok {
		return nil, false, false
	}

	for j := range p.raw {
		if seen&(1<<j) != 0 || p.raw[j].delete {
			continue
		}
		if emitted {
			replace(end, end, []byte(`,"`))
		} else {
			replace(end, end, []byte(`"`))
		}
		out = append(out, p.raw[j].key...)
		out = append(out, '"', ':')
		out = append(out, p.raw[j].value...)
		emitted = true
	}

	if out == nil {
		return doc, false, true
	}
	return append(out, doc[last:]...), true, true
}
