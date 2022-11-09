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

	"github.com/fulldump/inceptiondb/utils"
)

type Collection struct {
	Filename  string // Just informative...
	file      *os.File
	Rows      []*Row
	rowsMutex *sync.Mutex
	Indexes   map[string]*collectionIndex
	// buffer   *bufio.Writer // TODO: use write buffer to improve performance (x3 in tests)
}

type collectionIndex struct {
	Index
	Type    string
	Options interface{}
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
		Filename:  filename,
		Indexes:   map[string]*collectionIndex{},
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
			_, err := collection.addRow(command.Payload)
			if err != nil {
				return nil, err
			}
		case "index":
			indexCommand := &CreateIndexCommand{}
			json.Unmarshal(command.Payload, indexCommand) // Todo: handle error properly

			var options interface{}

			switch indexCommand.Type {
			case "map":
				options = &IndexMapOptions{}
				utils.Remarshal(indexCommand.Options, options)
			case "btree":
				options = &IndexBTreeOptions{}
				utils.Remarshal(indexCommand.Options, options)
			default:
				return nil, fmt.Errorf("index command: unexpected type '%s' instead of [map|btree]", indexCommand.Type)
			}

			err := collection.createIndex(indexCommand.Name, options, false)
			if err != nil {
				fmt.Printf("WARNING: create index '%s': %s\n", indexCommand.Name, err.Error())
			}
		case "remove":
			params := struct {
				I int
			}{}
			json.Unmarshal(command.Payload, &params) // Todo: handle error properly
			row := collection.Rows[params.I]         // this access is threadsafe, OpenCollection is a secuence
			err := collection.removeByRow(row, false)
			if err != nil {
				fmt.Printf("WARNING: remove row %d: %s\n", params.I, err.Error())
			}
		case "patch":
			params := struct {
				I    int
				Diff map[string]interface{}
			}{}
			json.Unmarshal(command.Payload, &params)
			row := collection.Rows[params.I] // this access is threadsafe, OpenCollection is a secuence
			err := collection.patchByRow(row, params.Diff, false)
			if err != nil {
				fmt.Printf("WARNING: patch item %d: %s\n", params.I, err.Error())
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

func (c *Collection) addRow(payload json.RawMessage) (*Row, error) {

	row := &Row{
		Payload: payload,
	}

	err := indexInsert(c.Indexes, row)
	if err != nil {
		return nil, err
	}

	c.rowsMutex.Lock()
	row.I = len(c.Rows)
	c.Rows = append(c.Rows, row)
	c.rowsMutex.Unlock()

	return row, nil
}

// TODO: test concurrency
func (c *Collection) Insert(item interface{}) (*Row, error) {
	if c.file == nil {
		return nil, fmt.Errorf("collection is closed")
	}

	payload, err := json.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("json encode payload: %w", err)
	}

	// Add row
	row, err := c.addRow(payload)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("json encode command: %w", err)
	}

	return row, nil
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

func (c *Collection) TraverseRange(from, to int, f func(row *Row)) { // todo: improve this naive  implementation
	for i, row := range c.Rows {
		if i < from {
			continue
		}
		if to > 0 && i >= to {
			break
		}
		f(row)
	}
}

type CreateIndexOptions struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

type CreateIndexCommand struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

// IndexMap create a unique index with a name
// Constraints: values can be only scalar strings or array of strings
func (c *Collection) Index(name string, options interface{}) error {
	return c.createIndex(name, options, true)
}

func (c *Collection) createIndex(name string, options interface{}, persist bool) error {

	if _, exists := c.Indexes[name]; exists {
		return fmt.Errorf("index '%s' already exists", name)
	}

	index := &collectionIndex{}

	switch value := options.(type) {
	case *IndexMapOptions:
		index.Type = "map"
		index.Index = NewIndexMap(value)
		index.Options = value
	case *IndexBTreeOptions:
		index.Type = "btree"
		index.Index = NewIndexBTree(value)
		index.Options = value
	default:
		return fmt.Errorf("unexpected options parameters, it should be [map|btree]")
	}

	c.Indexes[name] = index

	// Add all rows to the index
	for _, row := range c.Rows {
		err := index.AddRow(row)
		if err != nil {
			delete(c.Indexes, name)
			return fmt.Errorf("index row: %s, data: %s", err.Error(), string(row.Payload))
		}
	}

	if !persist {
		return nil
	}

	payload, err := json.Marshal(&CreateIndexCommand{
		Name:    name,
		Type:    index.Type,
		Options: options,
	})
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

func indexInsert(indexes map[string]*collectionIndex, row *Row) (err error) {
	for key, index := range indexes {
		err = index.AddRow(row)
		if err != nil {
			// TODO: undo previous work? two phases (check+commit) ?
			return fmt.Errorf("index add '%s': %s", key, err.Error())
		}
	}

	return
}

func indexRemove(indexes map[string]*collectionIndex, row *Row) (err error) {
	for key, index := range indexes {
		err = index.RemoveRow(row)
		if err != nil {
			// TODO: does this make any sense?
			return fmt.Errorf("index remove '%s': %s", key, err.Error())
		}
	}

	return
}

func (c *Collection) Remove(r *Row) error {
	return c.removeByRow(r, true)
}

// TODO: move this to utils/diogenesis?
func lockBlock(m *sync.Mutex, f func() error) error {
	m.Lock()
	defer m.Unlock()
	return f()
}

func (c *Collection) removeByRow(row *Row, persist bool) error { // todo: rename to 'removeRow'

	var i int
	err := lockBlock(c.rowsMutex, func() error {
		i = row.I
		if len(c.Rows) <= i {
			return fmt.Errorf("row %d does not exist", i)
		}

		err := indexRemove(c.Indexes, row)
		if err != nil {
			return fmt.Errorf("could not free index")
		}

		last := len(c.Rows) - 1
		c.Rows[i] = c.Rows[last]
		c.Rows[i].I = i
		c.Rows = c.Rows[:last]
		return nil
	})
	if err != nil {
		return err
	}

	if !persist {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"i": i,
	})
	if err != nil {
		return err // todo: wrap error
	}
	command := &Command{
		Name:      "remove",
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	err = json.NewEncoder(c.file).Encode(command)
	if err != nil {
		// TODO: panic?
		return fmt.Errorf("json encode command: %w", err)
	}

	return nil
}

func (c *Collection) Patch(row *Row, patch interface{}) error {
	return c.patchByRow(row, patch, true)
}

func (c *Collection) patchByRow(row *Row, patch interface{}, persist bool) error { // todo: rename to 'patchRow'

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
		"i":    row.I,
		"diff": json.RawMessage(diff),
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

	err = os.Remove(c.Filename)
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}

	return nil
}
