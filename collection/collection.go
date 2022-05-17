package collection

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"
)

type Collection struct {
	filename  string // Just informative...
	file      *os.File
	Rows      []*Row
	rowsMutex *sync.Mutex
	Indexes   map[string]Index
	//buffer   *bufio.Writer // TODO: use write buffer to improve performance (x3 in tests)
}

type Row struct {
	I       int // position in Rows
	Payload json.RawMessage
}

func OpenCollection(filename string) (*Collection, error) {

	// TODO: initialize, read all file and apply its changes into memory
	f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for read: %w", err)
	}

	collection := &Collection{
		Rows:      []*Row{},
		rowsMutex: &sync.Mutex{},
		filename:  filename,
		Indexes:   map[string]Index{},
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
			err := collection.addRow(command.Payload)
			if err != nil {
				return nil, err
			}
		case "index":
			options := &IndexOptions{}
			json.Unmarshal(command.Payload, options) // Todo: handle error properly
			err := collection.indexRows(options)
			if err != nil {
				fmt.Printf("WARNING: create index '%s': %s\n", options.Field, err.Error())
			}
		case "delete":
			filter := struct {
				Field string
				Value string
			}{}
			json.Unmarshal(command.Payload, &filter) // Todo: handle error properly
			err := collection.deleteRow(filter.Field, filter.Value, false)
			if err != nil {
				fmt.Printf("WARNING: delete item '%s'='%s': %s\n", filter.Field, filter.Value, err.Error())
			}
		case "patch":
			params := struct {
				Field string
				Value string
				Diff  map[string]interface{}
			}{}
			json.Unmarshal(command.Payload, &params)
			err := collection.patchRow(params.Field, params.Value, params.Diff, false)
			if err != nil {
				fmt.Printf("WARNING: patch item '%s'='%s': %s\n", params.Field, params.Value, err.Error())
			}
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

func (c *Collection) addRow(payload json.RawMessage) error {

	row := &Row{
		Payload: payload,
	}

	err := indexInsert(c.Indexes, row)
	if err != nil {
		return err
	}

	c.rowsMutex.Lock()
	row.I = len(c.Rows)
	c.Rows = append(c.Rows, row)
	c.rowsMutex.Unlock()

	return nil
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
	err = c.addRow(payload)
	if err != nil {
		return err
	}

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
	for _, row := range c.Rows {
		json.Unmarshal(row.Payload, data)
		return
	}
	// TODO return with error not found? or nil?
}

func (c *Collection) Traverse(f func(data []byte)) { // todo: return *Row instead of data?
	for _, row := range c.Rows {
		f(row.Payload)
	}
}

func (c *Collection) indexRows(options *IndexOptions) error {

	index := Index{
		Entries: map[string]*Row{},
		RWmutex: &sync.RWMutex{},
		Sparse:  options.Sparse, // include the whole `options` struct?
	}
	for _, row := range c.Rows {
		err := indexRow(index, options.Field, row)
		if err != nil {
			return fmt.Errorf("index row: %w, data: %s", err, string(row.Payload))
		}
	}
	c.Indexes[options.Field] = index

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

func indexInsert(indexes map[string]Index, row *Row) (err error) {
	for key, index := range indexes {
		err = indexRow(index, key, row)
		if err != nil {
			// TODO: undo previous work? two phases (check+commit) ?
			break
		}
	}

	return
}

func indexRow(index Index, field string, row *Row) error {

	item := map[string]interface{}{}

	err := json.Unmarshal(row.Payload, &item)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	itemValue, itemExists := item[field]
	if !itemExists {
		if index.Sparse {
			// Do not index
			return nil
		}
		return fmt.Errorf("field `%s` is indexed and mandatory", field)
	}

	switch value := itemValue.(type) {
	case string:

		index.RWmutex.RLock()
		_, exists := index.Entries[value]
		index.RWmutex.RUnlock()
		if exists {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
		}

		index.RWmutex.Lock()
		index.Entries[value] = row
		index.RWmutex.Unlock()
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			if _, exists := index.Entries[s]; exists {
				return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			index.Entries[s] = row
		}
	default:
		return fmt.Errorf("type not supported")
	}

	return nil
}

func indexRemove(indexes map[string]Index, row *Row) (err error) {
	for key, index := range indexes {
		err = unindexRow(index, key, row)
		if err != nil {
			// TODO: does this make any sense?
			break
		}
	}

	return
}

func unindexRow(index Index, field string, row *Row) error {

	item := map[string]interface{}{}

	err := json.Unmarshal(row.Payload, &item)
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
		delete(index.Entries, value)
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			delete(index.Entries, s)
		}
	default:
		// Should this error?
		return fmt.Errorf("type not supported")
	}

	return nil
}

func (c *Collection) FindBy(field string, value string, data interface{}) error {

	index, ok := c.Indexes[field]
	if !ok {
		return fmt.Errorf("field '%s' is not indexed", field)
	}

	row, ok := index.Entries[value]
	if !ok {
		return fmt.Errorf("%s '%s' not found", field, value)
	}

	return json.Unmarshal(row.Payload, &data)
}

func (c *Collection) DeleteBy(field string, value string) error {
	return c.deleteRow(field, value, true)
}

func (c *Collection) deleteRow(field string, value string, persist bool) error {

	index, ok := c.Indexes[field]
	if !ok {
		return fmt.Errorf("field '%s' is not indexed", field)
	}

	row, ok := index.Entries[value]
	if !ok {
		return fmt.Errorf("%s '%s' not found", field, value)
	}

	err := indexRemove(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("could not unindex %s='%v'", field, value)
	}

	c.rowsMutex.Lock()
	c.Rows[row.I] = c.Rows[len(c.Rows)-1]
	c.Rows = c.Rows[:len(c.Rows)-1]
	c.rowsMutex.Unlock()

	if !persist {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"field": field,
		"value": value,
	})
	if err != nil {
		return err // todo: wrap error
	}
	command := &Command{
		Name:      "delete",
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

func (c *Collection) PatchBy(field string, value string, patch map[string]interface{}) error {
	return c.patchRow(field, value, patch, true)
}

func (c *Collection) patchRow(field string, value string, patch map[string]interface{}, persist bool) error {

	index, ok := c.Indexes[field]
	if !ok {
		return fmt.Errorf("field '%s' is not indexed", field)
	}

	row, ok := index.Entries[value]
	if !ok {
		return fmt.Errorf("%s '%s' not found", field, value)
	}

	patchBytes, err := json.Marshal(patch)
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}

	newPayload, err := jsonpatch.MergePatch(row.Payload, patchBytes)
	if err != nil {
		return fmt.Errorf("cannot apply patch: %w", err)
	}

	diff, err := jsonpatch.CreateMergePatch(row.Payload, newPayload) // todo: optimization: discard operation if empty
	if err != nil {
		return fmt.Errorf("cannot diff: %w", err)
	}

	// index update
	err = indexRemove(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("indexRemove: %w", err)
	}

	row.Payload = newPayload

	err = indexInsert(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("indexInsert: %w", err)
	}

	if !persist {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"field": field,
		"value": value,
		"diff":  json.RawMessage(diff),
	})
	if err != nil {
		return err // todo: wrap error
	}
	command := &Command{
		Name:      "patch",
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
