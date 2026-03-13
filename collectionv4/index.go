package collectionv4

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/google/btree"
)

type Index interface {
	Add(id int64, data []byte) error
	Remove(id int64, data []byte) error
	GetType() string
	GetOptions() interface{}
}

// --- IndexMap ---

type IndexMap struct {
	Entries map[string]int64
	RWmutex *sync.RWMutex
	Options *IndexMapOptions
}

type IndexMapOptions struct {
	Field  string `json:"field"`
	Sparse bool   `json:"sparse"`
}

func NewIndexMap(options *IndexMapOptions) *IndexMap {
	return &IndexMap{
		Entries: map[string]int64{},
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (i *IndexMap) Remove(id int64, data []byte) error {
	var item map[string]any
	// TODO: Performance optimization bypassing map decoding
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in remove: %w", err)
	}

	field := i.Options.Field
	itemValue, itemExists := item[field]
	if !itemExists {
		return nil
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	switch value := itemValue.(type) {
	case string:
		delete(i.Entries, value)
	case []interface{}:
		for _, v := range value {
			if s, ok := v.(string); ok {
				delete(i.Entries, s)
			}
		}
	}

	return nil
}

func (i *IndexMap) Add(id int64, data []byte) error {
	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in add: %w", err)
	}

	field := i.Options.Field
	itemValue, itemExists := item[field]
	if !itemExists {
		if i.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field `%s` is indexed and mandatory", field)
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	switch value := itemValue.(type) {
	case string:
		if _, exists := i.Entries[value]; exists {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
		}
		i.Entries[value] = id

	case []interface{}:
		for _, v := range value {
			s, ok := v.(string)
			if !ok {
				continue
			}
			if _, exists := i.Entries[s]; exists {
				return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			if s, ok := v.(string); ok {
				i.Entries[s] = id
			}
		}
	default:
		return fmt.Errorf("type not supported by IndexMap")
	}

	return nil
}

func (i *IndexMap) GetType() string {
	return "map"
}

func (i *IndexMap) GetOptions() interface{} {
	return i.Options
}

// --- IndexBtree ---

type IndexBTreeOptions struct {
	Fields []string `json:"fields"`
	Sparse bool     `json:"sparse"`
	Unique bool     `json:"unique"`
}

type IndexBtree struct {
	Btree   *btree.BTreeG[*RowOrdered]
	Options *IndexBTreeOptions
}

type RowOrdered struct {
	ID     int64
	Values []interface{}
}

func NewIndexBTree(options *IndexBTreeOptions) *IndexBtree {
	index := btree.NewG(32, func(a, b *RowOrdered) bool {
		for i, valA := range a.Values {
			valB := b.Values[i]
			if reflect.DeepEqual(valA, valB) {
				continue
			}

			field := options.Fields[i]
			reverse := strings.HasPrefix(field, "-")

			switch valA := valA.(type) {
			case string:
				valB, ok := valB.(string)
				if !ok {
					continue
				}
				if reverse {
					return !(valA < valB)
				}
				return valA < valB

			case float64:
				valB, ok := valB.(float64)
				if !ok {
					continue
				}
				if reverse {
					return !(valA < valB)
				}
				return valA < valB
			}
		}
		return false
	})

	return &IndexBtree{
		Btree:   index,
		Options: options,
	}
}

func (b *IndexBtree) Remove(id int64, data []byte) error {
	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in remove (btree): %w", err)
	}

	values := make([]interface{}, 0, len(b.Options.Fields))
	for _, field := range b.Options.Fields {
		field = strings.TrimPrefix(field, "-")
		if v, exists := item[field]; exists {
			values = append(values, v)
		}
	}

	b.Btree.Delete(&RowOrdered{
		ID:     id,
		Values: values,
	})

	return nil
}

func (b *IndexBtree) Add(id int64, data []byte) error {
	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in add (btree): %w", err)
	}

	var values []interface{}
	for _, field := range b.Options.Fields {
		cleanField := strings.TrimPrefix(field, "-")
		value, exists := item[cleanField]
		if exists {
			values = append(values, value)
			continue
		}
		if b.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field '%s' not defined", cleanField)
	}

	if b.Options.Unique {
		if b.Btree.Has(&RowOrdered{Values: values}) {
			return fmt.Errorf("key already exists for unique btree index")
		}
	}

	b.Btree.ReplaceOrInsert(&RowOrdered{
		ID:     id,
		Values: values,
	})

	return nil
}

func (b *IndexBtree) GetType() string {
	return "btree"
}

func (b *IndexBtree) GetOptions() interface{} {
	return b.Options
}

// --- IndexFTS ---

type IndexFTS struct {
	Index   map[string]map[int64]struct{}
	RWmutex *sync.RWMutex
	Options *IndexFTSOptions
}

type IndexFTSOptions struct {
	Field string `json:"field"`
}

func NewIndexFTS(options *IndexFTSOptions) *IndexFTS {
	return &IndexFTS{
		Index:   map[string]map[int64]struct{}{},
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (i *IndexFTS) tokenize(text string) []string {
	text = strings.ToLower(text)
	return strings.Fields(text)
}

func (i *IndexFTS) Add(id int64, data []byte) error {
	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in add (fts): %w", err)
	}

	field := i.Options.Field
	value, exists := item[field]
	if !exists {
		return nil // Field missing, skip
	}

	strValue, ok := value.(string)
	if !ok {
		return nil // Not a string, skip
	}

	tokens := i.tokenize(strValue)

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	for _, token := range tokens {
		if _, ok := i.Index[token]; !ok {
			i.Index[token] = map[int64]struct{}{}
		}
		i.Index[token][id] = struct{}{}
	}

	return nil
}

func (i *IndexFTS) Remove(id int64, data []byte) error {
	var item map[string]interface{}
	if err := json.Unmarshal(data, &item); err != nil {
		return fmt.Errorf("unmarshal in remove (fts): %w", err)
	}

	field := i.Options.Field
	value, exists := item[field]
	if !exists {
		return nil
	}

	strValue, ok := value.(string)
	if !ok {
		return nil
	}

	tokens := i.tokenize(strValue)

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	for _, token := range tokens {
		if rows, ok := i.Index[token]; ok {
			delete(rows, id)
			if len(rows) == 0 {
				delete(i.Index, token)
			}
		}
	}

	return nil
}

func (i *IndexFTS) GetType() string {
	return "fts"
}

func (i *IndexFTS) GetOptions() interface{} {
	return i.Options
}
