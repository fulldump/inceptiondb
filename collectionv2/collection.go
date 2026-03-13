package collectionv2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/buger/jsonparser"
	"github.com/google/uuid"

	records "github.com/fulldump/inceptiondb/collectionv4/records"
)

type fastInserter interface {
	PersistInsert(seq uint64, timestamp int64, payload []byte) error
}

type Collection struct {
	Filename   string
	storage    Storage
	Rows       records.Records[*Row]
	mutex      *sync.RWMutex
	Indexes    map[string]Index
	Defaults   map[string]any
	Count      int64
	MaxID      int64        // Monotonic ID counter
	Seq        uint64       // Command sequence counter for fast UUID generation
	fastInsert fastInserter // cached interface for fast-path inserts
}

func OpenCollection(filename string) (*Collection, error) {
	// storage, err := NewSnapshotStorage(filename)
	storage, err := NewJSONStorage(filename)
	// storage, err := NewGobStorage(filename)
	// storage, err := NewWALStorage(filename)

	if err != nil {
		return nil, fmt.Errorf("open storage: %w", err)
	}

	c := &Collection{
		Filename: filename,
		storage:  storage,
		Rows:     records.NewRecordsUltra[*Row](),
		mutex:    &sync.RWMutex{},
		Indexes:  map[string]Index{},
	}

	// Cache fast-path inserter if storage supports it
	if fi, ok := storage.(fastInserter); ok {
		c.fastInsert = fi
	}

	// Load from storage
	err = LoadCollection(c)
	if err != nil {
		storage.Close()
		return nil, fmt.Errorf("load collection: %w", err)
	}

	return c, nil
}

func (c *Collection) Close() error {
	return c.storage.Close()
}

func (c *Collection) EncodeCommand(command *Command, id string, payload interface{}) error {
	return c.storage.Persist(command, id, payload)
}

func (c *Collection) InsertJSON(payload []byte) (*Row, error) {
	auto := atomic.AddInt64(&c.Count, 1)

	if len(c.Defaults) > 0 {
		changed := false
		var item map[string]any

		for k, v := range c.Defaults {
			_, _, _, err := jsonparser.Get(payload, k)
			if err == nil {
				continue // key already exists
			}

			// Key is missing, we need to add the default
			if !changed {
				changed = true
				item = map[string]any{}
				if uerr := json.Unmarshal(payload, &item); uerr != nil {
					return nil, fmt.Errorf("json decode payload: %w", uerr)
				}
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

		if changed {
			var err error
			payload, err = json.Marshal(item)
			if err != nil {
				return nil, fmt.Errorf("json encode payload: %w", err)
			}
		} else {
			payload = bytes.Clone(payload)
		}
	} else {
		payload = bytes.Clone(payload)
	}

	// Add row
	row := &Row{
		Payload: payload,
	}
	err := c.addRow(row)
	if err != nil {
		return nil, err
	}

	// Persist via fast path if available
	seq := atomic.AddUint64(&c.Seq, 1)
	ts := time.Now().UnixNano()
	if c.fastInsert != nil {
		err = c.fastInsert.PersistInsert(seq, ts, payload)
	} else {
		command := Command{
			Name:      "insert",
			Uuid:      strconv.FormatUint(seq, 36),
			Timestamp: ts,
			Payload:   payload,
		}
		err = c.EncodeCommand(&command, "", nil)
	}
	if err != nil {
		return nil, err
	}

	return row, nil
}

func (c *Collection) Insert(item map[string]any) (*Row, error) {
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
	row := &Row{
		Payload: payload,
	}
	err = c.addRow(row)
	if err != nil {
		return nil, err
	}

	// Persist
	command := &Command{
		Name:      "insert",
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	err = c.EncodeCommand(command, "", nil)
	if err != nil {
		return nil, err
	}

	return row, nil
}

func (c *Collection) addRow(row *Row) error {
	// Use monotonic ID
	id := atomic.AddInt64(&c.MaxID, 1)
	row.I = int(id)

	if len(c.Indexes) > 0 {
		c.mutex.RLock()
		err := indexInsert(c.Indexes, row)
		c.mutex.RUnlock()
		if err != nil {
			return err
		}
	}

	c.Rows.Set(int64(row.I), row)

	return nil
}

func (c *Collection) Remove(r *Row) error {
	return c.removeByRow(r, true)
}

func (c *Collection) removeByRow(row *Row, persist bool) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	row.PatchMutex.Lock()
	defer row.PatchMutex.Unlock()

	if !c.hasRow(row.I) {
		return fmt.Errorf("row %d does not exist", row.I)
	}

	err := indexRemove(c.Indexes, row)
	if err != nil {
		return fmt.Errorf("could not free index: %w", err)
	}

	// Capture ID before delete (SliceContainer might invalidate it)
	id := row.I

	c.Rows.Delete(int64(row.I))
	atomic.AddInt64(&c.Count, -1)

	if !persist {
		return nil
	}

	// Persist
	payload, err := json.Marshal(map[string]interface{}{
		"i": id,
	})
	if err != nil {
		return err
	}
	command := &Command{
		Name:      "remove",
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command, fmt.Sprintf("%d", id), nil)
}

func (c *Collection) Patch(row *Row, patch interface{}) error {
	return c.patchByRow(row, patch, true)
}

func (c *Collection) patchByRow(row *Row, patch interface{}, persist bool) error {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	row.PatchMutex.Lock()
	defer row.PatchMutex.Unlock()

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

	// Check if row still exists
	if !c.hasRow(row.I) {
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
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command, fmt.Sprintf("%d", row.I), newValue)
}

func (c *Collection) FindOne(data interface{}) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	// Just get the first one
	c.traverseRows(func(row *Row) bool {
		json.Unmarshal(row.Payload, data)
		return false // Stop after first
	})
}

func (c *Collection) Traverse(f func(data []byte)) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	c.traverseRows(func(row *Row) bool {
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
	case *IndexFTSOptions:
		index = NewIndexFTS(value)
	default:
		return fmt.Errorf("unexpected options parameters, it should be [map|btree|fts]")
	}

	c.Indexes[name] = index

	// Add all rows to the index
	var err error
	c.traverseRows(func(row *Row) bool {
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
	if _, ok := options.(*IndexFTSOptions); ok {
		typeStr = "fts"
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
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command, "", nil)
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
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command, "", nil)
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
		Uuid:      strconv.FormatUint(atomic.AddUint64(&c.Seq, 1), 36),
		Timestamp: time.Now().UnixNano(),
		StartByte: 0,
		Payload:   payload,
	}

	return c.EncodeCommand(command, "", nil)
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

func (c *Collection) hasRow(id int) bool {
	if id <= 0 {
		return false
	}
	return c.Rows.Get(int64(id)) != nil
}

func (c *Collection) getRow(id int) (*Row, bool) {
	if id <= 0 {
		return nil, false
	}
	row := c.Rows.Get(int64(id))
	if row == nil {
		return nil, false
	}
	return row, true
}

func (c *Collection) rowsLen() int {
	total := 0
	max := atomic.LoadInt64(&c.MaxID)
	for i := int64(1); i <= max; i++ {
		if c.Rows.Get(i) != nil {
			total++
		}
	}
	return total
}

func (c *Collection) traverseRows(iterator func(row *Row) bool) {
	max := atomic.LoadInt64(&c.MaxID)
	for i := int64(1); i <= max; i++ {
		row := c.Rows.Get(i)
		if row == nil {
			continue
		}
		if !iterator(row) {
			return
		}
	}
}
