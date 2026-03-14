package collectionv4

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/valyala/fastjson"
)

func (c *Collection) Patch(id int64, patch interface{}, wait bool) error { // nolint:gocyclo
	rec := c.records.Get(id)
	if !rec.Active {
		return fmt.Errorf("row %d does not exist", id)
	}

	var p fastjson.Parser
	v, err := p.ParseBytes(rec.Data)
	if err != nil {
		return fmt.Errorf("decode row payload: %w", err)
	}

	var arena fastjson.Arena
	merged, changed := fastjsonMergePatch(&arena, v, patch)

	if !changed {
		return nil
	}

	newPayload := merged.MarshalTo(nil)

	// Update record and indexes
	c.mu.RLock()
	indexRemove(c.indexes, id, rec.Data)
	c.mu.RUnlock()

	c.records.Set(id, Record{
		Data:   newPayload,
		Active: true,
	})

	c.mu.RLock()
	err = indexInsert(c.indexes, id, newPayload)
	c.mu.RUnlock()

	if err != nil {
		// Rollback memoria (no es 100% transaccional pero intentamos revertir)
		// ...
		return fmt.Errorf("indexInsert: %w", err)
	}

	// Persist partial diff logic
	// Pero en inceptiondb V4 el log es binario y soporta OpUpdate.
	// Podemos simplemente hacer Append del newPayload.
	if err := c.store.Append(OpUpdate, id, newPayload, wait); err != nil {
		return fmt.Errorf("journal write failed: %v", err)
	}

	return nil
}

// Update acts as a full payload replacement
func (c *Collection) Update(id int64, data []byte, wait bool) error {
	rec := c.records.Get(id)
	if !rec.Active {
		return fmt.Errorf("row %d does not exist", id)
	}

	c.mu.RLock()
	indexRemove(c.indexes, id, rec.Data)
	c.mu.RUnlock()

	c.records.Set(id, Record{
		Data:   data,
		Active: true,
	})

	c.mu.RLock()
	err := indexInsert(c.indexes, id, data)
	c.mu.RUnlock()

	if err != nil {
		return fmt.Errorf("indexInsert: %w", err)
	}

	if err := c.store.Append(OpUpdate, id, data, wait); err != nil {
		return fmt.Errorf("journal write failed: %v", err)
	}

	return nil
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
