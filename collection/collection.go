package collection

import (
	"encoding/json"
	"io"
	"os"
)

type Collection struct {
	file     *os.File
	rows     []json.RawMessage
	filename string // Just informative...
}

func OpenCollection(filename string) *Collection {

	// TODO: initialize, read all file and apply its changes into memory
	rows := []json.RawMessage{}
	f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}
	j := json.NewDecoder(f)
	for {
		row := json.RawMessage{}
		err := j.Decode(&row)
		if err == io.EOF {
			break
		}
		if err != nil {
			panic(err)
		}
		rows = append(rows, row)
	}

	// Open file for append only
	// todo: investigate O_SYNC
	f, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		// TODO: handle or return error
		panic(err)
	}

	return &Collection{
		file:     f,
		rows:     rows,
		filename: filename,
	}
}

// TODO: test concurrency
func (c *Collection) Insert(data interface{}) {
	if c.file == nil {
		panic("Collection is already closed!")
	}
	json.NewEncoder(c.file).Encode(data)
}

func (c *Collection) FindOne(data interface{}) {
	for _, row := range c.rows {
		json.Unmarshal(row, data)
		return
	}
	// TODO return with error not found? or nil?
}

func (c *Collection) Close() error {
	err := c.file.Close()
	c.file = nil
	return err
}
