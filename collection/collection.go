package collection

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type Collection struct {
	file *os.File
	//buffer   *bufio.Writer // TODO: use write buffer to improve performance (x3 in tests)
	rows     []json.RawMessage
	filename string // Just informative...
	indexes  map[string]Index
}

type Index map[string]json.RawMessage

func OpenCollection(filename string) *Collection {

	// TODO: initialize, read all file and apply its changes into memory
	f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		panic(err)
	}

	rows := []json.RawMessage{}
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
		indexes:  map[string]Index{},
	}
}

// TODO: test concurrency
func (c *Collection) Insert(item interface{}) error {
	if c.file == nil {
		return fmt.Errorf("collection is closed")
	}

	data, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("json encode: %w", err)
	}

	// update indexes
	indexInsert(c.indexes, data)

	c.rows = append(c.rows, data)
	c.file.Write(data)
	c.file.WriteString("\n")

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

// Index create a unique index with a name
// Constraints: values can be only scalar strings or array of strings
func (c *Collection) Index(field string) error {

	index := Index{}
	for _, rowData := range c.rows {
		err := indexRow(index, field, rowData)
		if err != nil {
			return fmt.Errorf("index row: %w, data: %s", err, string(rowData))
		}
	}

	c.indexes[field] = index

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
