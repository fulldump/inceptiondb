package collection

import (
	"bufio"
	"bytes"
	"encoding/json"
	"encoding/json/jsontext"
	json2 "encoding/json/v2"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	jsonpatch "github.com/evanphx/json-patch"
	"github.com/google/uuid"

	"github.com/fulldump/inceptiondb/utils"
)

type Collection struct {
	Filename     string // Just informative...
	file         *os.File
	Rows         []*Row
	rowsMutex    *sync.Mutex
	Indexes      map[string]*collectionIndex // todo: protect access with mutex or use sync.Map
	buffer       *bufio.Writer               // TODO: use write buffer to improve performance (x3 in tests)
	Defaults     map[string]any
	Count        int64
	encoderMutex *sync.Mutex
}

type collectionIndex struct {
	Index
	Type    string
	Options interface{}
}

type Row struct {
	I          int // position in Rows
	Payload    json.RawMessage
	PatchMutex sync.Mutex
}

type EncoderMachine struct {
	Buffer *bytes.Buffer
	Enc    *json.Encoder
	Enc2   *jsontext.Encoder
}

var encPool = sync.Pool{
	New: func() any {
		buffer := bytes.NewBuffer(make([]byte, 0, 8*1024))
		enc := json.NewEncoder(buffer)
		enc.SetEscapeHTML(false)
		return &EncoderMachine{
			Buffer: buffer,
			Enc:    enc,
			Enc2: jsontext.NewEncoder(
				buffer,
				jsontext.AllowDuplicateNames(true),
				jsontext.AllowDuplicateNames(true),
				jsontext.EscapeForHTML(false),
				jsontext.Multiline(false),
				jsontext.EscapeForJS(false),
				jsontext.ReorderRawObjects(false),
			),
		}
	},
}

func OpenCollection(filename string) (*Collection, error) {

	// TODO: initialize, read all file and apply its changes into memory
	f, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for read: %w", err)
	}

	collection := &Collection{
		Rows:         []*Row{},
		rowsMutex:    &sync.Mutex{},
		Filename:     filename,
		Indexes:      map[string]*collectionIndex{},
		encoderMutex: &sync.Mutex{},
	}

	j := jsontext.NewDecoder(f,
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
	)

	command := &Command{}

	for {
		command.Payload = nil

		err := json2.UnmarshalDecode(j, &command)
		if err == io.EOF {
			break
		}
		if err != nil {
			// todo: try a best effort?
			return nil, fmt.Errorf("decode json: %w", err)
		}

		switch command.Name {
		case "insert":
			_, err := collection.addRow(command.Payload, nil)
			if err != nil {
				return nil, err
			}
		case "drop_index":
			dropIndexCommand := &DropIndexCommand{}
			json.Unmarshal(command.Payload, dropIndexCommand) // Todo: handle error properly

			err := collection.dropIndex(dropIndexCommand.Name, false)
			if err != nil {
				fmt.Printf("WARNING: drop index '%s': %s\n", dropIndexCommand.Name, err.Error())
				// TODO: stop process? if error might get inconsistent state
			}
		case "index": // todo: rename to create_index
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
		case "set_defaults":
			defaults := map[string]any{}
			json.Unmarshal(command.Payload, &defaults)
			collection.setDefaults(defaults, false)
		}
	}

	// Open file for append only
	// todo: investigate O_SYNC
	collection.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	collection.buffer = bufio.NewWriterSize(collection.file, 16*1024*1024)

	return collection, nil
}

func (c *Collection) addRow(payload json.RawMessage, item map[string]any) (*Row, error) {

	row := &Row{
		Payload: payload,
	}

	err := indexInsert(c.Indexes, row, item)
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
func (c *Collection) Insert(item map[string]any) (*Row, error) {
	if c.file == nil {
		return nil, fmt.Errorf("collection is closed")
	}

	auto := atomic.AddInt64(&c.Count, 1)

	if c.Defaults != nil {
		for k, v := range c.Defaults {
			if item[k] != nil {
				continue
			}
			var value any
			switch v {
			case "uuid()":
				value = uuid.NewString()
			case "unixnano()":
				value = time.Now().UnixNano()
			case "auto()":
				value = auto
			default:
				value = v
			}
			item[k] = value
		}
	}

	payload, err := json2.Marshal(item)
	if err != nil {
		return nil, fmt.Errorf("json encode payload: %w", err)
	}

	// Add row
	row, err := c.addRow(payload, item)
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

	err = c.EncodeCommand(command)
	if err != nil {
		return nil, err
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

func (c *Collection) SetDefaults(defaults map[string]any) error {
	return c.setDefaults(defaults, true)
}

func (c *Collection) setDefaults(defaults map[string]any, persist bool) error {

	c.Defaults = defaults

	if !persist {
		return nil
	}

	payload, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	command := &Command{
		Name:      "set_defaults", // todo: rename to create_index
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command)
}

// IndexMap create a unique index with a name
// Constraints: values can be only scalar strings or array of strings
func (c *Collection) Index(name string, options interface{}) error { // todo: rename to CreateIndex
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
		index.Index = NewIndexSyncMap(value)
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
		item := map[string]any{}
		json2.Unmarshal(row.Payload, &item)
		err := index.AddRow(row, item)
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
		Name:      "index", // todo: rename to create_index
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command)
}

func indexInsert(indexes map[string]*collectionIndex, row *Row, item map[string]any) (err error) {

	if item == nil {
		item = make(map[string]any)
		json2.Unmarshal(row.Payload, &item)
	}

	// Note: rollbacks array should be kept in stack if it is smaller than 65536 bytes, so
	// our recommended maximum number of indexes should NOT exceed 8192 indexes

	rollbacks := make([]*collectionIndex, len(indexes))
	c := 0

	defer func() {
		if err == nil {
			return
		}
		for i := 0; i < c; i++ {
			rollbacks[i].RemoveRow(row, item)
		}
	}()

	for key, index := range indexes {
		err = index.AddRow(row, item)
		if err != nil {
			return fmt.Errorf("index add '%s': %s", key, err.Error())
		}

		rollbacks[c] = index
		c++
	}

	return
}

func indexRemove(indexes map[string]*collectionIndex, row *Row, item map[string]any) (err error) {

	for key, index := range indexes {
		err = index.RemoveRow(row, item)
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

	item := map[string]any{}
	json2.Unmarshal(row.Payload, &item)

	var i int
	err := lockBlock(c.rowsMutex, func() error {
		i = row.I
		if len(c.Rows) <= i {
			return fmt.Errorf("row %d does not exist", i)
		}

		err := indexRemove(c.Indexes, row, item)
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

	return c.EncodeCommand(command)
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

	if len(diff) == 2 { // diff == '{}'
		return nil
	}

	item := map[string]any{}
	json2.Unmarshal(row.Payload, &item)

	// index update
	err = indexRemove(c.Indexes, row, item)
	if err != nil {
		return fmt.Errorf("indexRemove: %w", err)
	}

	row.Payload = newPayload

	err = indexInsert(c.Indexes, row, item)
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

	return c.EncodeCommand(command)
}

func (c *Collection) Close() error {
	{
		err := c.buffer.Flush()
		if err != nil {
			return err
		}
	}

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

func (c *Collection) DropIndex(name string) error {
	return c.dropIndex(name, true)
}

type DropIndexCommand struct {
	Name string `json:"name"`
}

func (c *Collection) dropIndex(name string, persist bool) error {
	_, exists := c.Indexes[name]
	if !exists {
		return fmt.Errorf("dropIndex: index '%s' not found", name)
	}
	delete(c.Indexes, name)

	if !persist {
		return nil
	}

	payload, err := json.Marshal(&CreateIndexCommand{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	command := &Command{
		Name:      "drop_index",
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command)
}

func (c *Collection) EncodeCommand(command *Command) error {

	em := encPool.Get().(*EncoderMachine)
	defer encPool.Put(em)
	em.Buffer.Reset()

	// err := em.Enc.Encode(command)
	err := json2.MarshalEncode(em.Enc2, command)
	// err := json2.MarshalWrite(em.Buffer, command)
	if err != nil {
		return err
	}

	b := em.Buffer.Bytes()
	c.encoderMutex.Lock()
	c.buffer.Write(b)
	//	c.file.Write(b)
	c.encoderMutex.Unlock()
	return nil
}
