package collectionv2

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/google/btree"
)

// --- IndexMap ---

type IndexMap struct {
	Entries map[string]*Row
	RWmutex *sync.RWMutex
	Options *IndexMapOptions
}

type IndexMapOptions struct {
	Field  string `json:"field"`
	Sparse bool   `json:"sparse"`
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
	if row.Decoded != nil {
		item = row.Decoded.(map[string]interface{})
	} else {
		err := json.Unmarshal(row.Payload, &item)
		if err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}

	field := i.Options.Field
	entries := i.Entries

	itemValue, itemExists := item[field]
	if !itemExists {
		return nil
	}

	switch value := itemValue.(type) {
	case string:
		delete(entries, value)
	case []interface{}:
		for _, v := range value {
			s := v.(string) // TODO: handle casting error
			delete(entries, s)
		}
	default:
		return fmt.Errorf("type not supported")
	}

	return nil
}

func (i *IndexMap) AddRow(row *Row) error {
	item := map[string]interface{}{}
	if row.Decoded != nil {
		item = row.Decoded.(map[string]interface{})
	} else {
		err := json.Unmarshal(row.Payload, &item)
		if err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
	}

	field := i.Options.Field
	itemValue, itemExists := item[field]
	if !itemExists {
		if i.Options.Sparse {
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
			s := v.(string)
			if _, exists := entries[s]; exists {
				return fmt.Errorf("index conflict: field '%s' with value '%s'", field, value)
			}
		}
		for _, v := range value {
			s := v.(string)
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
	json.Unmarshal(optionsData, options)

	i.RWmutex.RLock()
	row, ok := i.Entries[options.Value]
	i.RWmutex.RUnlock()
	if !ok {
		return
	}

	f(row)
}

func (i *IndexMap) GetType() string {
	return "map"
}

func (i *IndexMap) GetOptions() interface{} {
	return i.Options
}

// --- IndexBTree ---

type IndexBtree struct {
	Btree   *btree.BTreeG[*RowOrdered]
	Options *IndexBTreeOptions
}

type IndexBTreeOptions struct {
	Fields []string `json:"fields"`
	Sparse bool     `json:"sparse"`
	Unique bool     `json:"unique"`
}

type RowOrdered struct {
	*Row
	Values []interface{}
}

// Less implements btree.Item
func (r *RowOrdered) Less(than *RowOrdered) bool {
	// This comparison logic depends on how the BTree was initialized.
	// Since we can't access the BTree's less function here easily without passing it,
	// we might need to rethink this or duplicate the logic.
	// However, google/btree's ReplaceOrInsert uses the BTree's Less function.
	// But wait, `btree.NewG` takes a Less function.
	// So `RowOrdered` doesn't strictly need a `Less` method if we provide one to `NewG`.
	// The existing implementation in `collection/indexbtree.go` defined the Less logic in `NewIndexBTree`.
	return false // Dummy, logic is in NewIndexBTree
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
			// field = strings.TrimPrefix(field, "-") // Not used here

			switch valA := valA.(type) {
			case string:
				valB, ok := valB.(string)
				if !ok {
					panic("Type B should be string")
				}
				if reverse {
					return !(valA < valB)
				}
				return valA < valB

			case float64:
				valB, ok := valB.(float64)
				if !ok {
					panic("Type B should be float64")
				}
				if reverse {
					return !(valA < valB)
				}
				return valA < valB
			default:
				panic("Type A not supported")
			}
		}
		return false
	})

	return &IndexBtree{
		Btree:   index,
		Options: options,
	}
}

func (b *IndexBtree) RemoveRow(r *Row) error {
	values := []interface{}{}
	data := map[string]interface{}{}
	if r.Decoded != nil {
		data = r.Decoded.(map[string]interface{})
	} else {
		json.Unmarshal(r.Payload, &data)
	}

	for _, field := range b.Options.Fields {
		field = strings.TrimPrefix(field, "-")
		values = append(values, data[field])
	}

	b.Btree.Delete(&RowOrdered{
		Row:    r,
		Values: values,
	})

	return nil
}

func (b *IndexBtree) AddRow(r *Row) error {
	var values []interface{}
	data := map[string]interface{}{}
	if r.Decoded != nil {
		data = r.Decoded.(map[string]interface{})
	} else {
		json.Unmarshal(r.Payload, &data)
	}

	for _, field := range b.Options.Fields {
		field = strings.TrimPrefix(field, "-")
		value, exists := data[field]
		if exists {
			values = append(values, value)
			continue
		}
		if b.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field '%s' not defined", field)
	}

	if b.Btree.Has(&RowOrdered{Values: values}) {
		// Construct error key
		errKey := ""
		for i, field := range b.Options.Fields {
			pair := fmt.Sprint(field, ":", values[i])
			if errKey != "" {
				errKey += "," + pair
			} else {
				errKey = pair
			}
		}
		return fmt.Errorf("key (%s) already exists", errKey)
	}

	b.Btree.ReplaceOrInsert(&RowOrdered{
		Row:    r,
		Values: values,
	})

	return nil
}

type IndexBtreeTraverse struct {
	Reverse bool                   `json:"reverse"`
	From    map[string]interface{} `json:"from"`
	To      map[string]interface{} `json:"to"`
}

func (b *IndexBtree) Traverse(optionsData []byte, f func(*Row) bool) {
	options := &IndexBtreeTraverse{}
	json.Unmarshal(optionsData, options)

	iterator := func(r *RowOrdered) bool {
		return f(r.Row)
	}

	hasFrom := len(options.From) > 0
	hasTo := len(options.To) > 0

	pivotFrom := &RowOrdered{}
	if hasFrom {
		for _, field := range b.Options.Fields {
			field = strings.TrimPrefix(field, "-")
			pivotFrom.Values = append(pivotFrom.Values, options.From[field])
		}
	}

	pivotTo := &RowOrdered{}
	if hasTo {
		for _, field := range b.Options.Fields {
			field = strings.TrimPrefix(field, "-")
			pivotTo.Values = append(pivotTo.Values, options.To[field])
		}
	}

	if !hasFrom && !hasTo {
		if options.Reverse {
			b.Btree.Descend(iterator)
		} else {
			b.Btree.Ascend(iterator)
		}
	} else if hasFrom && !hasTo {
		if options.Reverse {
			b.Btree.DescendGreaterThan(pivotFrom, iterator)
		} else {
			b.Btree.AscendGreaterOrEqual(pivotFrom, iterator)
		}
	} else if !hasFrom && hasTo {
		if options.Reverse {
			b.Btree.DescendLessOrEqual(pivotTo, iterator)
		} else {
			b.Btree.AscendLessThan(pivotTo, iterator)
		}
	} else {
		if options.Reverse {
			b.Btree.DescendRange(pivotTo, pivotFrom, iterator)
		} else {
			b.Btree.AscendRange(pivotFrom, pivotTo, iterator)
		}
	}
}

func (b *IndexBtree) GetType() string {
	return "btree"
}

func (b *IndexBtree) GetOptions() interface{} {
	return b.Options
}

// --- IndexFTS ---

type IndexFTS struct {
	// Inverted index: token -> set of rows
	Index   map[string]map[*Row]struct{}
	RWmutex *sync.RWMutex
	Options *IndexFTSOptions
}

type IndexFTSOptions struct {
	Field string `json:"field"`
}

func NewIndexFTS(options *IndexFTSOptions) *IndexFTS {
	return &IndexFTS{
		Index:   map[string]map[*Row]struct{}{},
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (i *IndexFTS) tokenize(text string) []string {
	// Simple tokenizer: lowercase and split by space
	// TODO: Improve tokenizer (remove punctuation, stop words, etc.)
	text = strings.ToLower(text)
	return strings.Fields(text)
}

func (i *IndexFTS) AddRow(row *Row) error {
	item := map[string]interface{}{}
	if row.Decoded != nil {
		item = row.Decoded.(map[string]interface{})
	} else {
		err := json.Unmarshal(row.Payload, &item)
		if err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
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
			i.Index[token] = map[*Row]struct{}{}
		}
		i.Index[token][row] = struct{}{}
	}

	return nil
}

func (i *IndexFTS) RemoveRow(row *Row) error {
	item := map[string]interface{}{}
	if row.Decoded != nil {
		item = row.Decoded.(map[string]interface{})
	} else {
		err := json.Unmarshal(row.Payload, &item)
		if err != nil {
			return fmt.Errorf("unmarshal: %w", err)
		}
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
			delete(rows, row)
			if len(rows) == 0 {
				delete(i.Index, token)
			}
		}
	}

	return nil
}

type IndexFTSTraverse struct {
	Match string `json:"match"`
}

func (i *IndexFTS) Traverse(optionsData []byte, f func(row *Row) bool) {
	options := &IndexFTSTraverse{}
	json.Unmarshal(optionsData, options)

	tokens := i.tokenize(options.Match)
	if len(tokens) == 0 {
		return
	}

	// For now, just match the first token (OR logic? AND logic?)
	// Let's implement simple single-token match or intersection of all tokens?
	// "Match" usually implies the query string.
	// Let's do intersection (AND) of all tokens in the query.

	i.RWmutex.RLock()
	defer i.RWmutex.RUnlock()

	// Start with the set of rows for the first token
	firstToken := tokens[0]
	rows, ok := i.Index[firstToken]
	if !ok {
		return
	}

	// Copy to a candidate set to avoid locking issues or modifying the index?
	// Actually, we just need to iterate.
	// But we need to intersect with other tokens.

	// Optimization: start with the smallest set?
	// For now, just iterate the first set and check others.

	for row := range rows {
		matchAll := true
		for _, token := range tokens[1:] {
			if otherRows, ok := i.Index[token]; !ok {
				matchAll = false
				break
			} else {
				if _, exists := otherRows[row]; !exists {
					matchAll = false
					break
				}
			}
		}

		if matchAll {
			if !f(row) {
				return
			}
		}
	}
}

func (i *IndexFTS) GetType() string {
	return "fts"
}

func (i *IndexFTS) GetOptions() interface{} {
	return i.Options
}
