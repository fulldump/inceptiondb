package storage

import (
	"encoding/json"
)

type Command struct {
	Name      string          `json:"name"`
	Uuid      string          `json:"uuid"`
	Timestamp int64           `json:"timestamp"`
	StartByte int64           `json:"start_byte"`
	Payload   json.RawMessage `json:"payload"`
}

type CreateIndexCommand struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

type DropIndexCommand struct {
	Name string `json:"name"`
}

type LoadedCommand struct {
	Seq            int
	Cmd            *Command
	DecodedPayload interface{}
	Err            error
}

type Storage interface {
	// Persist persists a command.
	// id: the stable identifier of the row (if applicable, e.g. for insert/patch/remove)
	// payload: the current full value of the row (for insert/patch)
	Persist(cmd *Command, id string, payload interface{}) error
	Load() (<-chan LoadedCommand, <-chan error)
	Close() error
}
