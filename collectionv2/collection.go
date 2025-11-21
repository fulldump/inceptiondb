package collectionv2

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
)

type Collection struct {
	Filename     string
	file         *os.File
	Rows         RowContainer
	mutex        *sync.RWMutex
	Indexes      map[string]Index
	buffer       *bufio.Writer
	Defaults     map[string]any
	Count        int64
	MaxID        int64 // Monotonic ID counter
	commandQueue chan *Command
	closed       chan struct{}
	closeOnce    sync.Once
	wg           sync.WaitGroup
}

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

func OpenCollection(filename string) (*Collection, error) {
	c := &Collection{
		Filename:     filename,
		Rows:         NewSliceContainer(),
		mutex:        &sync.RWMutex{},
		Indexes:      map[string]Index{},
		commandQueue: make(chan *Command, 1000), // Buffer for async writes
		closed:       make(chan struct{}),
	}

	// Load from file (parallel deserialization)
	err := LoadCollection(filename, c)
	if err != nil {
		return nil, fmt.Errorf("load collection: %w", err)
	}

	// Open file for append
	c.file, err = os.OpenFile(filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return nil, fmt.Errorf("open file for write: %w", err)
	}

	c.buffer = bufio.NewWriterSize(c.file, 16*1024*1024)

	// Start background writer
	c.wg.Add(1)
	go c.writerLoop()

	return c, nil
}

func (c *Collection) writerLoop() {
	defer c.wg.Done()
	for {
		select {
		case cmd, ok := <-c.commandQueue:
			if !ok {
				return
			}
			c.writeCommand(cmd)
		case <-c.closed:
			// Drain queue
			for {
				select {
				case cmd := <-c.commandQueue:
					c.writeCommand(cmd)
				default:
					return
				}
			}
		}
	}
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func (c *Collection) writeCommand(cmd *Command) {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	enc := json.NewEncoder(buf)
	enc.SetEscapeHTML(false)
	enc.Encode(cmd)

	c.buffer.Write(buf.Bytes())
}

func (c *Collection) Close() error {
	c.closeOnce.Do(func() {
		close(c.closed)
	})
	c.wg.Wait()
	c.buffer.Flush()
	return c.file.Close()
}

func (c *Collection) EncodeCommand(command *Command) error {
	select {
	case c.commandQueue <- command:
		return nil
	case <-c.closed:
		return fmt.Errorf("collection closed")
	}
}

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

	err = c.EncodeCommand(command)
	if err != nil {
		return nil, err
	}

	return row, nil
}

func (c *Collection) addRow(payload json.RawMessage) (*Row, error) {
	row := &Row{
		Payload: payload,
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Use monotonic ID
	id := atomic.AddInt64(&c.MaxID, 1)
	row.I = int(id)

	err := indexInsert(c.Indexes, row)
	if err != nil {
		return nil, err
	}

	c.Rows.ReplaceOrInsert(row)

	return row, nil
}

func (c *Collection) Remove(r *Row) error {
	return c.removeByRow(r, true)
}

func (c *Collection) removeByRow(row *Row, persist bool) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.Rows.Has(row) {
		return fmt.Errorf("row %d does not exist", row.I)
	}

	err := indexRemove(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("could not free index: %w", err)
	}

	c.Rows.Delete(row)

	if !persist {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"i": row.I,
	})
	if err != nil {
		return err
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

func (c *Collection) patchByRow(row *Row, patch interface{}, persist bool) error {
	originalValue, err := decodeJSONValue(row.Payload)
	if err != nil {
		return fmt.Errorf("decode row payload: %w", err)
	}

	normalizedPatch, err := normalizeJSONValue(patch)
	if err != nil {
		return fmt.Errorf("normalize patch: %w", err)
	}

	newValue, changed, err := applyMergePatchValue(originalValue, normalizedPatch)
	if err != nil {
		return fmt.Errorf("cannot apply patch: %w", err)
	}

	if !changed {
		return nil
	}

	newPayload, err := json.Marshal(newValue)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	// Critical section for index update
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Check if row still exists
	if !c.Rows.Has(row) {
		return fmt.Errorf("row %d does not exist", row.I)
	}

	err = indexRemove(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("indexRemove: %w", err)
	}

	// Update payload
	// Note: This modifies the row in place. Since BTree stores pointers, this is reflected in the tree.
	// However, if the index depends on the payload, we need to re-insert into index.
	row.Payload = newPayload

	err = indexInsert(c.Indexes, row)
	if err != nil {
		// Rollback payload if index insert fails?
		// This is tricky. We should probably check index constraints before modifying row.
		// But indexInsert checks constraints.
		// If it fails, we are in a bad state: row has new payload but not in index.
		// We should try to revert payload and re-insert into index.
		// TODO: Implement rollback for patch
		return fmt.Errorf("indexInsert: %w", err)
	}

	if !persist {
		return nil
	}

	diffValue, hasDiff := createMergeDiff(originalValue, newValue)
	if !hasDiff {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"i":    row.I,
		"diff": diffValue,
	})
	if err != nil {
		return err
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

func (c *Collection) FindOne(data interface{}) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Just get the first one
	c.Rows.Traverse(func(row *Row) bool {
		json.Unmarshal(row.Payload, data)
		return false // Stop after first
	})
}

func (c *Collection) Traverse(f func(data []byte)) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.Rows.Traverse(func(row *Row) bool {
		f(row.Payload)
		return true
	})
}

func (c *Collection) Index(name string, options interface{}) error {
	return c.createIndex(name, options, true)
}

func (c *Collection) createIndex(name string, options interface{}, persist bool) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if _, exists := c.Indexes[name]; exists {
		return fmt.Errorf("index '%s' already exists", name)
	}

	var index Index

	switch value := options.(type) {
	case *IndexMapOptions:
		index = NewIndexMap(value)
	case *IndexBTreeOptions:
		index = NewIndexBTree(value)
	default:
		return fmt.Errorf("unexpected options parameters, it should be [map|btree]")
	}

	c.Indexes[name] = index

	// Add all rows to the index
	var err error
	c.Rows.Traverse(func(row *Row) bool {
		err = index.AddRow(row)
		if err != nil {
			return false // Stop
		}
		return true
	})

	if err != nil {
		delete(c.Indexes, name)
		return fmt.Errorf("index row: %w", err)
	}

	if !persist {
		return nil
	}

	// Determine type string
	typeStr := "map"
	if _, ok := options.(*IndexBTreeOptions); ok {
		typeStr = "btree"
	}

	payload, err := json.Marshal(&CreateIndexCommand{
		Name:    name,
		Type:    typeStr,
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

	return c.EncodeCommand(command)
}

func (c *Collection) DropIndex(name string) error {
	return c.dropIndex(name, true)
}

func (c *Collection) dropIndex(name string, persist bool) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	_, exists := c.Indexes[name]
	if !exists {
		return fmt.Errorf("dropIndex: index '%s' not found", name)
	}
	delete(c.Indexes, name)

	if !persist {
		return nil
	}

	payload, err := json.Marshal(&DropIndexCommand{
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
		Name:      "set_defaults",
		Uuid:      uuid.New().String(),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command)
}

func indexInsert(indexes map[string]Index, row *Row) (err error) {
	rollbacks := make([]Index, 0, len(indexes))

	defer func() {
		if err == nil {
			return
		}
		for _, index := range rollbacks {
			index.RemoveRow(row)
		}
	}()

	for key, index := range indexes {
		err = index.AddRow(row)
		if err != nil {
			return fmt.Errorf("index add '%s': %s", key, err.Error())
		}
		rollbacks = append(rollbacks, index)
	}

	return
}

func indexRemove(indexes map[string]Index, row *Row) (err error) {
	for key, index := range indexes {
		err = index.RemoveRow(row)
		if err != nil {
			return fmt.Errorf("index remove '%s': %s", key, err.Error())
		}
	}
	return
}
