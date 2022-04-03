package collection

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	file *os.File
	//buffer   *bufio.Writer // TODO: use write buffer to improve performance (x3 in tests)
	rows     []json.RawMessage
	filename string // Just informative...
	indexes  map[string]Index
}

func OpenCollection(filename string) (*Collection, error) {

	// TODO: initialize, read all file and apply its changes into memory
	f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for read: %w", err)
	}

	collection := &Collection{
		rows:     []json.RawMessage{},
		filename: filename,
		indexes:  map[string]Index{},
	}

	j := json.NewDecoder(f)
	for {
		command := &Command{}
		err := j.Decode(&command)
		if err == io.EOF {
			break
		}
		if err != nil {
			// todo: try a best effort?
			return nil, fmt.Errorf("decode json: %w", err)
		}

		switch command.Name {
		case "insert":
			collection.addRow(command.Payload)
		case "index":
			options := &IndexOptions{}
			json.Unmarshal(command.Payload, options)
			collection.indexRows(options)
		}
	}

	// Open file for append only
	// todo: investigate O_SYNC
	collection.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	return collection, nil
}

func (c *Collection) addRow(payload json.RawMessage) {
	indexInsert(c.indexes, payload)
	c.rows = append(c.rows, payload)
}

// TODO: test concurrency
func (c *Collection) Insert(item interface{}) error {
	if c.file == nil {
		return fmt.Errorf("collection is closed")
	}

	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	// Add row
	c.addRow(payload)

	// Persist
	command := &Command{
		Name:      "insert",
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	err = json.NewEncoder(c.file).Encode(command)
	if err != nil {
		return fmt.Errorf("json encode command: %w", err)
	}

	return nil
}

func (c *Collection) FindOne(data interface{}) {
	for _, row := range c.rows {
		json.Unmarshal(row, data)
		return
	}
	// TODO return with error not found? or nil?
}

func (c *Collection) Traverse(f func(data []byte)) {
	for _, row := range c.rows {
		f(row)
	}
}

func (c *Collection) indexRows(options *IndexOptions) error {

	index := Index{}
	for _, rowData := range c.rows {
		err := indexRow(index, options.Field, rowData)
		if err != nil {
			return fmt.Errorf("index row: %w, data: %s", err, string(rowData))
		}
	}
	c.indexes[options.Field] = index

	return nil
}

// Index create a unique index with a name
// Constraints: values can be only scalar strings or array of strings
func (c *Collection) Index(options *IndexOptions) error {

	err := c.indexRows(options)
	if err != nil {
		return err
	}

	payload, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	command := &Command{
		Name:      "index",
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	err = json.NewEncoder(c.file).Encode(command)
	if err != nil {
		return fmt.Errorf("json encode command: %w", err)
	}

	return nil
}

func indexInsert(indexes map[string]Index, rowData []byte) (err error) {
	for key, index := range indexes {
		err = indexRow(index, key, rowData)
		if err != nil {
			// TODO: undo previous work? two phases (check+commit) ?
			break
		}

	}

	return
}

func indexRow(index Index, field string, rowData []byte) error {

	item := map[string]interface{}{}

	err := json.Unmarshal(rowData, &item)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	itemValue, itemExists := item[field]
	if !itemExists {
		// Do not index
		return nil
	}

	switch value := itemValue.(type) {
	case string:
		if _, exists := index[value]; exists {
			return fmt.Errorf("conflict: field '%s' with value '%s'", field, value)
		}
		index[value] = rowData
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			if _, exists := index[s]; exists {
				return fmt.Errorf("conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			index[s] = rowData
		}
	default:
		return fmt.Errorf("type not supported")
	}

	return nil
}

func (c *Collection) FindBy(field string, value string, data interface{}) error {

	index, ok := c.indexes[field]
	if !ok {
		return fmt.Errorf("field '%s' is not indexed", field)
	}

	row, ok := index[value]
	if !ok {
		return fmt.Errorf("%s '%s' not found", field, value)
	}

	return json.Unmarshal(row, &data)
}

func (c *Collection) Close() error {
	err := c.file.Close()
	c.file = nil
	return err
}

func (c *Collection) Drop() error {
	err := c.Close()
	if err != nil {
		return fmt.Errorf("close: %w", err)
	}

	err = os.Remove(c.filename)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	return nil
}
