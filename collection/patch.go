package collection

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/fulldump/inceptiondb/collection/stores"
	"github.com/fulldump/inceptiondb/simdscan"
	"github.com/valyala/fastjson"
)

// ErrPatchNotObject rejects a patch that is not a JSON object. As a merge
// patch (RFC 7386) it would replace the whole document with that value.
var ErrPatchNotObject = errors.New("patch must be a JSON object")

type PatchPlan struct {
	Patch    interface{}
	topLevel map[string]patchField

	raw         []rawPatchField // see compileRaw
	rawEligible bool
	rawExtra    int // bytes the raw path may add to a document
}

type patchField struct {
	raw    []byte
	delete bool
}

func CompilePatch(patch interface{}) (*PatchPlan, error) {
	if plan, ok := patch.(*PatchPlan); ok {
		return plan, nil
	}

	plan := &PatchPlan{Patch: patch}
	if raw, ok := patch.(json.RawMessage); ok {
		if trimmed := bytes.TrimSpace(raw); len(trimmed) == 0 || trimmed[0] != '{' {
			return nil, ErrPatchNotObject
		}
		decoded, err := decodeRawPatch(raw)
		if err != nil {
			return nil, err
		}
		plan.Patch = decoded
		if err := plan.compileTopLevelRaw(raw); err != nil {
			return nil, err
		}
		plan.compileRaw(json.RawMessage(bytes.TrimSpace(raw)))
		return plan, nil
	}

	if patchMap, ok := patch.(map[string]interface{}); ok {
		plan.topLevel = make(map[string]patchField, len(patchMap))
		for k, v := range patchMap {
			if v == nil {
				plan.topLevel[k] = patchField{delete: true}
				continue
			}
			raw, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			plan.topLevel[k] = patchField{raw: raw}
		}
		plan.compileRaw(patchMap)
		return plan, nil
	}

	// Any other Go value (a struct, a string...) is checked by its JSON form
	raw, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("marshal patch: %w", err)
	}
	return CompilePatch(json.RawMessage(raw))
}

func decodeRawPatch(raw json.RawMessage) (interface{}, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var decoded interface{}
	if err := dec.Decode(&decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func (p *PatchPlan) compileTopLevelRaw(raw json.RawMessage) error {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return err
	}
	p.topLevel = make(map[string]patchField, len(fields))
	for k, v := range fields {
		value := bytes.TrimSpace(v)
		p.topLevel[k] = patchField{
			raw:    append([]byte(nil), value...),
			delete: bytes.Equal(value, []byte("null")),
		}
	}
	return nil
}

func (c *Collection) Patch(id int64, patch interface{}, wait bool) error {
	if _, err := c.PatchDeferred(id, patch); err != nil {
		return err
	}
	return c.Commit(wait)
}

// PatchDeferred patches without waiting for durability, see Commit. It
// returns the document after the patch.
func (c *Collection) PatchDeferred(id int64, patch interface{}) ([]byte, error) { // nolint:gocyclo
	plan, err := CompilePatch(patch)
	if err != nil {
		return nil, fmt.Errorf("compile patch: %w", err)
	}

	mu := c.stripe(id)
	mu.Lock()
	defer mu.Unlock()

	if c.closed {
		return nil, ErrCollectionClosed
	}

	rec := c.records.Get(id)
	if !rec.Active {
		return nil, fmt.Errorf("row %d does not exist", id)
	}

	if plan.rawEligible {
		if patched, changed, ok := plan.applyRaw(rec.Data); ok {
			if !changed {
				return rec.Data, nil
			}
			if err := c.replaceLocked(id, rec.Data, patched); err != nil {
				return nil, err
			}
			return patched, nil
		}
	}

	if changed, ok := patchPlanChangesTopLevel(rec.Data, plan); ok && !changed {
		return rec.Data, nil
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(rec.Data)
	if err != nil {
		return nil, fmt.Errorf("decode row payload: %w", err)
	}

	var arena fastjson.Arena
	merged, changed := fastjsonMergePatch(&arena, v, plan.Patch)

	if !changed {
		return rec.Data, nil
	}

	patched := merged.MarshalTo(nil)
	if err := c.replaceLocked(id, rec.Data, patched); err != nil {
		return nil, err
	}
	return patched, nil
}

// replaceLocked swaps the payload of an active record. The caller holds the
// stripe of id. On error the record and its indexes are left unchanged.
func (c *Collection) replaceLocked(id int64, oldPayload, newPayload []byte) error {
	idxMap := *c.indexes.Load()
	reserved, hasAsync, err := indexReserveSync(idxMap, id, oldPayload, newPayload)
	if err != nil {
		return fmt.Errorf("indexInsert: %w", err)
	}

	if err := c.store.Append(stores.OpUpdate, id, newPayload, false); err != nil {
		indexReleaseSync(reserved, id, newPayload, oldPayload)
		return fmt.Errorf("journal write failed: %w", err)
	}

	indexReleaseSync(reserved, id, oldPayload, newPayload)
	c.records.Set(id, Record{
		Data:   newPayload,
		Active: true,
	})

	if hasAsync {
		c.asyncIndexOp(stores.OpUpdate, id, append([]byte(nil), newPayload...), append([]byte(nil), oldPayload...))
	}

	return nil
}

func patchPlanChangesTopLevel(data []byte, plan *PatchPlan) (bool, bool) {
	if plan == nil || plan.topLevel == nil {
		return true, false
	}
	for key, field := range plan.topLevel {
		current, typ, err := simdscan.GetField(data, key)
		if field.delete {
			if err == nil {
				return true, true
			}
			continue
		}
		if err != nil || !rawPatchValueEqual(current, typ, field.raw) {
			return true, true
		}
	}
	return false, true
}

func rawPatchValueEqual(current []byte, typ simdscan.Type, expected []byte) bool {
	expected = bytes.TrimSpace(expected)
	if typ != simdscan.TypeString {
		return bytes.Equal(current, expected)
	}
	if len(expected) < 2 || expected[0] != '"' {
		return false
	}
	unquoted, err := strconv.Unquote(string(expected))
	if err != nil {
		return false
	}
	return bytes.Equal(current, []byte(unquoted))
}

// Update acts as a full payload replacement
func (c *Collection) Update(id int64, data []byte, wait bool) error {
	mu := c.stripe(id)
	mu.Lock()
	if c.closed {
		mu.Unlock()
		return ErrCollectionClosed
	}
	rec := c.records.Get(id)
	if !rec.Active {
		mu.Unlock()
		return fmt.Errorf("row %d does not exist", id)
	}
	err := c.replaceLocked(id, rec.Data, data)
	mu.Unlock()
	if err != nil {
		return err
	}
	return c.Commit(wait)
}

func fastjsonMergePatch(arena *fastjson.Arena, original *fastjson.Value, patch interface{}) (*fastjson.Value, bool) {
	if raw, ok := patch.(json.RawMessage); ok {
		var decoded interface{}
		if err := json.Unmarshal(raw, &decoded); err == nil {
			patch = decoded
		}
	}

	if patchMap, ok := patch.(map[string]interface{}); ok {
		changed := false
		if original == nil || original.Type() != fastjson.TypeObject {
			original = arena.NewObject()
			changed = true
		}

		for k, v := range patchMap {
			if v == nil {
				if original.Get(k) != nil {
					original.Del(k)
					changed = true
				}
			} else {
				origVal := original.Get(k)
				merged, valChanged := fastjsonMergePatch(arena, origVal, v)
				if valChanged || origVal == nil {
					original.Set(k, merged)
					changed = true
				}
			}
		}
		return original, changed
	}

	newVal := buildFastjsonValue(arena, patch)
	if original == nil {
		return newVal, true
	}

	// Compare bytes to detect if it really changed
	if bytes.Equal(original.MarshalTo(nil), newVal.MarshalTo(nil)) {
		return original, false
	}
	return newVal, true
}

func buildFastjsonValue(arena *fastjson.Arena, val interface{}) *fastjson.Value {
	switch v := val.(type) {
	case string:
		return arena.NewString(v)
	case json.Number:
		return arena.NewNumberString(string(v))
	case int:
		return arena.NewNumberInt(v)
	case int8:
		return arena.NewNumberInt(int(v))
	case int16:
		return arena.NewNumberInt(int(v))
	case int32:
		return arena.NewNumberInt(int(v))
	case int64:
		return arena.NewNumberString(strconv.FormatInt(v, 10))
	case uint:
		return arena.NewNumberString(strconv.FormatUint(uint64(v), 10))
	case uint8:
		return arena.NewNumberInt(int(v))
	case uint16:
		return arena.NewNumberInt(int(v))
	case uint32:
		return arena.NewNumberInt(int(v))
	case uint64:
		return arena.NewNumberString(strconv.FormatUint(v, 10))
	case float32:
		return arena.NewNumberFloat64(float64(v))
	case float64:
		return arena.NewNumberFloat64(v)
	case bool:
		if v {
			return arena.NewTrue()
		}
		return arena.NewFalse()
	case nil:
		return arena.NewNull()
	case []interface{}:
		arr := arena.NewArray()
		for i, item := range v {
			if raw, ok := item.(json.RawMessage); ok {
				var decoded interface{}
				_ = json.Unmarshal(raw, &decoded)
				item = decoded
			}
			arr.SetArrayItem(i, buildFastjsonValue(arena, item))
		}
		return arr
	case map[string]interface{}:
		obj := arena.NewObject()
		for k, item := range v {
			if raw, ok := item.(json.RawMessage); ok {
				var decoded interface{}
				_ = json.Unmarshal(raw, &decoded)
				item = decoded
			}
			obj.Set(k, buildFastjsonValue(arena, item))
		}
		return obj
	case json.RawMessage:
		var decoded interface{}
		if err := json.Unmarshal(v, &decoded); err == nil {
			return buildFastjsonValue(arena, decoded)
		}
		return arena.NewNull()
	default:
		// Fallback for custom structs or unhandled types
		b, err := json.Marshal(v)
		if err == nil {
			var decoded interface{}
			if err := json.Unmarshal(b, &decoded); err == nil {
				return buildFastjsonValue(arena, decoded)
			}
		}
		return arena.NewNull()
	}
}
