package collection

import "encoding/json"

type Command struct {
	Name      string          `json:"name"`
	Uuid      string          `json:"uuid"`
	Timestamp int64           `json:"timestamp"`
	StartByte int64           `json:"start_byte"`
	Payload   json.RawMessage `json:"payload"`
}
