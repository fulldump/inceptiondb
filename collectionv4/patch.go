package collectionv4

import (
	"encoding/json"
	"fmt"
	"reflect"
)

func (c *Collection) Patch(id int64, patch interface{}) error { // nolint:gocyclo
	rec := c.records.Get(id)
	if !rec.Active {
		return fmt.Errorf("row %d does not exist", id)
	}

	originalValue, err := decodeJSONValue(rec.Data)
	if err != nil {
		return fmt.Errorf("decode row payload: %w", err)
	}

	normalizedPatch, err := normalizeJSONValue(patch)
	if err != nil {
		return fmt.Errorf("normalize patch: %w", err)
	}

	newValue, changed, err := applyMergePatchValue(originalValue, normalizedPatch)
	if err != nil {
		return fmt.Errorf("cannot apply patch: %w", err)
	}

	if !changed {
		return nil
	}

	newPayload, err := json.Marshal(newValue)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

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
	if err := c.store.Append(OpUpdate, id, newPayload); err != nil {
		return fmt.Errorf("journal write failed: %v", err)
	}

	return nil
}

// Update acts as a full payload replacement
func (c *Collection) Update(id int64, data []byte) error {
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

	if err := c.store.Append(OpUpdate, id, data); err != nil {
		return fmt.Errorf("journal write failed: %v", err)
	}

	return nil
}

func decodeJSONValue(raw []byte) (interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func normalizeJSONValue(value interface{}) (interface{}, error) {
	switch v := value.(type) {
	case json.RawMessage:
		var decoded interface{}
		if err := json.Unmarshal(v, &decoded); err != nil {
			return nil, err
		}
		return normalizeJSONValue(decoded)
	case map[string]interface{}:
		normalized := make(map[string]interface{}, len(v))
		for key, item := range v {
			nv, err := normalizeJSONValue(item)
			if err != nil {
				return nil, err
			}
			normalized[key] = nv
		}
		return normalized, nil
	case []interface{}:
		normalized := make([]interface{}, len(v))
		for i, item := range v {
			nv, err := normalizeJSONValue(item)
			if err != nil {
				return nil, err
			}
			normalized[i] = nv
		}
		return normalized, nil
	default:
		return v, nil
	}
}

func applyMergePatchValue(original interface{}, patch interface{}) (interface{}, bool, error) {
	switch p := patch.(type) {
	case map[string]interface{}:
		var originalMap map[string]interface{}
		if m, ok := original.(map[string]interface{}); ok {
			originalMap = m
		}

		result := make(map[string]interface{}, len(originalMap)+len(p))
		for k, v := range originalMap {
			result[k] = cloneJSONValue(v)
		}

		changed := false
		for k, item := range p {
			if item == nil {
				if _, exists := result[k]; exists {
					delete(result, k)
					changed = true
				}
				continue
			}

			originalValue := interface{}(nil)
			if originalMap != nil {
				originalValue, _ = originalMap[k]
			}

			mergedValue, valueChanged, err := applyMergePatchValue(originalValue, item)
			if err != nil {
				return nil, false, err
			}

			if originalMap == nil {
				changed = true
			} else {
				if _, exists := originalMap[k]; !exists || valueChanged {
					changed = true
				}
			}

			result[k] = mergedValue
		}

		return result, changed, nil
	case []interface{}:
		cloned := cloneJSONArray(p)
		if current, ok := original.([]interface{}); ok {
			if reflect.DeepEqual(current, cloned) {
				return cloned, false, nil
			}
		}
		return cloned, true, nil
	default:
		if reflect.DeepEqual(original, p) {
			return cloneJSONValue(p), false, nil
		}
		return cloneJSONValue(p), true, nil
	}
}

func cloneJSONValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		cloned := make(map[string]interface{}, len(v))
		for k, item := range v {
			cloned[k] = cloneJSONValue(item)
		}
		return cloned
	case []interface{}:
		return cloneJSONArray(v)
	case json.RawMessage:
		if v == nil {
			return nil
		}
		cloned := make(json.RawMessage, len(v))
		copy(cloned, v)
		return cloned
	default:
		return v
	}
}

func cloneJSONArray(values []interface{}) []interface{} {
	if values == nil {
		return nil
	}
	cloned := make([]interface{}, len(values))
	for i, item := range values {
		cloned[i] = cloneJSONValue(item)
	}
	return cloned
}
