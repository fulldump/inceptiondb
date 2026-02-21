package collectionv2

import (
	"encoding/json"
	"sync"
)

type Row struct {
	I          int // position in Rows, used as ID
	Payload    json.RawMessage
	Decoded    interface{}
	PatchMutex sync.Mutex
}

// Less returns true if the row is less than the other row.
// This is required for btree.Item interface.
func (r *Row) Less(than *Row) bool {
	return r.I < than.I
}
