package collection

import (
	"encoding/json"
	"fmt"
	"sync"
)

// IndexSyncMap should be an interface to allow multiple kinds and implementations
type IndexSyncMap struct {
	Entries *sync.Map
	Options *IndexMapOptions
}

func NewIndexSyncMap(options *IndexMapOptions) *IndexSyncMap {
	return &IndexSyncMap{
		Entries: &sync.Map{},
		Options: options,
	}
}

func (i *IndexSyncMap) RemoveRow(row *Row) error {

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
		entries.Delete(value)
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			entries.Delete(s)
		}
	default:
		// Should this error?
		return fmt.Errorf("type not supported")
	}

	return nil
}

func (i *IndexSyncMap) AddRow(row *Row) error {

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

	entries := i.Entries

	switch value := itemValue.(type) {
	case string:
		_, exists := entries.Load(value)
		if exists {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
		}
		entries.Store(value, row)
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			if _, exists := entries.Load(s); exists {
				return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			s := v.(string) // TODO: handle this casting error
			entries.Store(s, row)
		}
	default:
		return fmt.Errorf("type not supported")
	}

	return nil
}

type IndexSyncMapTraverse struct {
	Value string `json:"value"`
}

func (i *IndexSyncMap) Traverse(optionsData []byte, f func(row *Row) bool) {

	options := &IndexMapTraverse{}
	json.Unmarshal(optionsData, options) // todo: handle error

	row, ok := i.Entries.Load(options.Value)
	if !ok {
		return
	}

	f(row.(*Row))
}
