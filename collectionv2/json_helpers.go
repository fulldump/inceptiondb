package collectionv2

import (
	"encoding/json"
	"reflect"
)

func decodeJSONValue(raw json.RawMessage) (interface{}, error) {
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

func createMergeDiff(original interface{}, modified interface{}) (interface{}, bool) {
	switch o := original.(type) {
	case map[string]interface{}:
		modifiedMap, ok := modified.(map[string]interface{})
		if !ok {
			if reflect.DeepEqual(original, modified) {
				return nil, false
			}
			return cloneJSONValue(modified), true
		}

		diff := make(map[string]interface{})
		changed := false

		for k := range o {
			if _, exists := modifiedMap[k]; !exists {
				diff[k] = nil
				changed = true
			}
		}

		for k, mv := range modifiedMap {
			ov, exists := o[k]
			if !exists {
				diff[k] = cloneJSONValue(mv)
				changed = true
				continue
			}

			if om, ok := ov.(map[string]interface{}); ok {
				if mm, ok := mv.(map[string]interface{}); ok {
					subDiff, subChanged := createMergeDiff(om, mm)
					if subChanged {
						diff[k] = subDiff
						changed = true
					}
					continue
				}
			}

			if oa, ok := ov.([]interface{}); ok {
				if ma, ok := mv.([]interface{}); ok {
					if !reflect.DeepEqual(oa, ma) {
						diff[k] = cloneJSONValue(mv)
						changed = true
					}
					continue
				}
			}

			if !reflect.DeepEqual(ov, mv) {
				diff[k] = cloneJSONValue(mv)
				changed = true
			}
		}

		if !changed {
			return nil, false
		}
		return diff, true
	case []interface{}:
		if ma, ok := modified.([]interface{}); ok {
			if reflect.DeepEqual(o, ma) {
				return nil, false
			}
			return cloneJSONValue(ma), true
		}
		if reflect.DeepEqual(original, modified) {
			return nil, false
		}
		return cloneJSONValue(modified), true
	default:
		if reflect.DeepEqual(original, modified) {
			return nil, false
		}
		return cloneJSONValue(modified), true
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
