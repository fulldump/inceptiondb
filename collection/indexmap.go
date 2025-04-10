package collection

import (
	"encoding/json"
	"fmt"
	"sync"
)

// IndexMap should be an interface to allow multiple kinds and implementations
type IndexMap struct {
	Entries map[string]*Row
	RWmutex *sync.RWMutex
	Options *IndexMapOptions
}

func NewIndexMap(options *IndexMapOptions) *IndexMap {
	return &IndexMap{
		Entries: map[string]*Row{},
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (i *IndexMap) RemoveRow(row *Row) error {

	item := map[string]interface{}{}

	err := json.Unmarshal(row.Payload, &item)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	field := i.Options.Field
	entries := i.Entries

	itemValue, itemExists := item[field]
	if !itemExists {
		// Do not index
		return nil
	}

	switch value := itemValue.(type) {
	case string:
		delete(entries, value)
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			delete(entries, s)
		}
	default:
		// Should this error?
		return fmt.Errorf("type not supported")
	}

	return nil
}

func (i *IndexMap) AddRow(row *Row) error {

	item := map[string]interface{}{}
	err := json.Unmarshal(row.Payload, &item)
	if err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}

	field := i.Options.Field

	itemValue, itemExists := item[field]
	if !itemExists {
		if i.Options.Sparse {
			// Do not index
			return nil
		}
		return fmt.Errorf("field `%s` is indexed and mandatory", field)
	}

	mutex := i.RWmutex
	entries := i.Entries

	switch value := itemValue.(type) {
	case string:

		mutex.RLock()
		_, exists := entries[value]
		mutex.RUnlock()
		if exists {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
		}

		mutex.Lock()
		entries[value] = row
		mutex.Unlock()

	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			if _, exists := entries[s]; exists {
				return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			entries[s] = row
		}
	default:
		return fmt.Errorf("type not supported")
	}

	return nil
}

type IndexMapTraverse struct {
	Value string `json:"value"`
}

func (i *IndexMap) Traverse(optionsData []byte, f func(row *Row) bool) {

	options := &IndexMapTraverse{}
	json.Unmarshal(optionsData, options) // todo: handle error

	i.RWmutex.RLock()
	row, ok := i.Entries[options.Value]
	i.RWmutex.RUnlock()
	if !ok {
		return
	}

	f(row)
}

// IndexMapOptions should have attributes like unique, sparse, multikey, sorted, background, etc...
// IndexMap should be an interface to have multiple indexes implementations, key value, B-Tree, bitmap, geo, cache...
type IndexMapOptions struct {
	Field  string `json:"field"`
	Sparse bool   `json:"sparse"`
}
