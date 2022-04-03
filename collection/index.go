package collection

import "encoding/json"

// Index should be an interface to allow multiple kinds and implementations
type Index map[string]json.RawMessage

// IndexOptions should have attributes like unique, sparse, multikey, sorted, background, etc...
// Index should be an interface to have multiple indexes implementations, key value, B-Tree, bitmap, geo, cache...
type IndexOptions struct {
	Field string `json:"field"`
}
