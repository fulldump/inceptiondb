package collection

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/buger/jsonparser"
	"github.com/google/btree"

	"github.com/fulldump/inceptiondb/simdscan"
)

// simdToInterface converts a simdscan value+type to a Go interface{}
// compatible with the BTree comparator (which expects string or float64).
func simdToInterface(val []byte, t simdscan.Type) interface{} {
	switch t {
	case simdscan.TypeString:
		return string(val)
	case simdscan.TypeNumber:
		f, err := strconv.ParseFloat(string(val), 64)
		if err != nil {
			return string(val)
		}
		return f
	case simdscan.TypeBoolean:
		return val[0] == 't'
	default:
		return string(val)
	}
}

type Index interface {
	Add(id int64, data []byte) error
	Remove(id int64, data []byte) error
	Traverse(options []byte, f func(id int64, data []byte) bool)
	GetType() string
	GetOptions() interface{}
	IsUnique() bool
	// Size estimates the memory taken by the index, in O(1)
	Size() int64
}

// indexEntryOverhead is a rough estimate of what an entry costs in a Go
// container besides its key: headers, pointers and buckets.
const indexEntryOverhead = 48

// --- IndexMap ---

type IndexMap struct {
	Entries map[string]*IndexMapEntry
	RWmutex *sync.RWMutex
	Options *IndexMapOptions

	keyBytes int64 // guarded by RWmutex
}

type IndexMapEntry struct {
	ID int64
}

type IndexMapOptions struct {
	Field  string `json:"field"`
	Sparse bool   `json:"sparse"`
}

func NewIndexMap(options *IndexMapOptions) *IndexMap {
	return &IndexMap{
		Entries: map[string]*IndexMapEntry{},
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (i *IndexMap) Remove(id int64, data []byte) error {
	keys, _, err := i.keys(data)
	if err != nil {
		return nil
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	i.removeKeys(id, keys)
	return nil
}

// keys returns the index keys of a document. indexed is false when the field
// is missing.
func (i *IndexMap) keys(data []byte) (keys []string, indexed bool, err error) {
	field := i.Options.Field
	val, dt, err := simdscan.GetField(data, field)
	if err != nil {
		return nil, false, nil
	}

	switch dt {
	case simdscan.TypeString:
		return []string{string(val)}, true, nil
	case simdscan.TypeArray:
		jsonparser.ArrayEach(data, func(value []byte, adt jsonparser.ValueType, offset int, err error) {
			if adt == jsonparser.String {
				keys = append(keys, string(value))
			}
		}, field)
		return keys, true, nil
	default:
		return nil, true, fmt.Errorf("type not supported by IndexMap")
	}
}

// removeKeys deletes the keys owned by id. Must be called with the lock held.
func (i *IndexMap) removeKeys(id int64, keys []string) {
	for _, k := range keys {
		if entry, ok := i.Entries[k]; ok && entry.ID == id {
			delete(i.Entries, k)
			i.keyBytes -= int64(len(k))
		}
	}
}

// conflict returns an error if any key is owned by a document other than id.
// Must be called with the lock held.
func (i *IndexMap) conflict(id int64, keys []string) error {
	for _, k := range keys {
		if entry, ok := i.Entries[k]; ok && entry.ID != id {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", i.Options.Field, k)
		}
	}
	return nil
}

func (i *IndexMap) Add(id int64, data []byte) error {
	keys, indexed, err := i.keys(data)
	if err != nil {
		return err
	}
	if !indexed {
		if i.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field `%s` is indexed and mandatory", i.Options.Field)
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	if err := i.conflict(id, keys); err != nil {
		return err
	}
	for _, k := range keys {
		if _, exists := i.Entries[k]; !exists {
			i.keyBytes += int64(len(k))
		}
		i.Entries[k] = &IndexMapEntry{ID: id}
	}

	return nil
}

// Release removes the keys of oldData owned by id that newData no longer uses.
func (i *IndexMap) Release(id int64, oldData, newData []byte) {
	oldKeys, _, _ := i.keys(oldData)
	newKeys, _, _ := i.keys(newData)

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	for _, k := range oldKeys {
		if !slices.Contains(newKeys, k) {
			i.removeKeys(id, []string{k})
		}
	}
}

type IndexMapTraverse struct {
	Value string `json:"value"`
}

func (i *IndexMap) Traverse(optionsData []byte, f func(id int64, data []byte) bool) {
	options := &IndexMapTraverse{}
	_ = json.Unmarshal(optionsData, options)

	i.RWmutex.RLock()
	entry, ok := i.Entries[options.Value]
	i.RWmutex.RUnlock()
	if !ok {
		return
	}

	f(entry.ID, nil)
}

func (i *IndexMap) GetType() string {
	return "map"
}

func (i *IndexMap) GetOptions() interface{} {
	return i.Options
}

func (i *IndexMap) IsUnique() bool {
	return true
}

func (i *IndexMap) Size() int64 {
	i.RWmutex.RLock()
	defer i.RWmutex.RUnlock()
	return int64(len(i.Entries))*indexEntryOverhead + i.keyBytes
}

// --- IndexBtree ---

type IndexBTreeOptions struct {
	Fields []string `json:"fields"`
	Sparse bool     `json:"sparse"`
	Unique bool     `json:"unique"`
}

type IndexBtree struct {
	Btree   *btree.BTreeG[*RowOrdered]
	RWmutex *sync.RWMutex
	bytes   int64 // guarded by RWmutex
	Options *IndexBTreeOptions
}

type RowOrdered struct {
	ID     int64
	Values []interface{}
}

type boundMarker uint8

const (
	boundLow boundMarker = iota + 1
	boundHigh
)

func compareFieldValues(a, b interface{}, reverse bool) int {
	if markerA, ok := a.(boundMarker); ok {
		switch markerA {
		case boundLow:
			if markerB, ok := b.(boundMarker); ok && markerB == boundLow {
				return 0
			}
			return -1
		case boundHigh:
			if markerB, ok := b.(boundMarker); ok && markerB == boundHigh {
				return 0
			}
			return 1
		}
	}
	if markerB, ok := b.(boundMarker); ok {
		switch markerB {
		case boundLow:
			return 1
		case boundHigh:
			return -1
		}
	}

	cmp := 0
	switch va := a.(type) {
	case string:
		vb, ok := b.(string)
		if ok {
			cmp = strings.Compare(va, vb)
			break
		}
		cmp = strings.Compare(fmt.Sprintf("%T:%v", a, a), fmt.Sprintf("%T:%v", b, b))

	case float64:
		vb, ok := b.(float64)
		if ok {
			switch {
			case va < vb:
				cmp = -1
			case va > vb:
				cmp = 1
			default:
				cmp = 0
			}
			break
		}
		cmp = strings.Compare(fmt.Sprintf("%T:%v", a, a), fmt.Sprintf("%T:%v", b, b))

	case bool:
		vb, ok := b.(bool)
		if ok {
			switch {
			case va == vb:
				cmp = 0
			case !va && vb:
				cmp = -1
			default:
				cmp = 1
			}
			break
		}
		cmp = strings.Compare(fmt.Sprintf("%T:%v", a, a), fmt.Sprintf("%T:%v", b, b))

	case nil:
		if b == nil {
			cmp = 0
		} else {
			cmp = -1
		}

	default:
		cmp = strings.Compare(fmt.Sprintf("%T:%v", a, a), fmt.Sprintf("%T:%v", b, b))
	}

	if reverse {
		return -cmp
	}
	return cmp
}

func compareTupleValues(options *IndexBTreeOptions, a, b []interface{}) int {
	for i, field := range options.Fields {
		var va, vb interface{}
		if i < len(a) {
			va = a[i]
		}
		if i < len(b) {
			vb = b[i]
		}

		reverse := strings.HasPrefix(field, "-")
		cmp := compareFieldValues(va, vb, reverse)
		if cmp != 0 {
			return cmp
		}
	}

	return 0
}

func NewIndexBTree(options *IndexBTreeOptions) *IndexBtree {
	index := btree.NewG(32, func(a, b *RowOrdered) bool {
		cmp := compareTupleValues(options, a.Values, b.Values)
		if cmp != 0 || options.Unique {
			return cmp < 0
		}
		// Non-unique: documents sharing the same key are kept apart by id
		return a.ID < b.ID
	})

	return &IndexBtree{
		Btree:   index,
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

// values extracts the indexed tuple. indexed is false when a field is missing.
func (b *IndexBtree) values(data []byte) (values []interface{}, indexed bool, missing string) {
	values = make([]interface{}, 0, len(b.Options.Fields))
	for _, field := range b.Options.Fields {
		cleanField := strings.TrimPrefix(field, "-")
		val, dt, err := simdscan.GetField(data, cleanField)
		if err != nil {
			return nil, false, cleanField
		}
		values = append(values, simdToInterface(val, dt))
	}
	return values, true, ""
}

// removeLocked deletes the entry of id. Must be called with the lock held.
func (b *IndexBtree) removeLocked(id int64, values []interface{}) {
	item := &RowOrdered{ID: id, Values: values}
	if b.Options.Unique {
		// Unique entries are keyed by value only: never remove another document's entry
		if found, ok := b.Btree.Get(item); !ok || found.ID != id {
			return
		}
	}
	if deleted, ok := b.Btree.Delete(item); ok {
		b.bytes -= rowOrderedSize(deleted)
	}
}

// rowOrderedSize estimates the memory of a btree entry.
func rowOrderedSize(r *RowOrdered) int64 {
	size := int64(indexEntryOverhead)
	for _, v := range r.Values {
		size += 16 // interface header
		if s, ok := v.(string); ok {
			size += int64(len(s))
		} else {
			size += 8
		}
	}
	return size
}

func (b *IndexBtree) Remove(id int64, data []byte) error {
	values, indexed, _ := b.values(data)
	if !indexed {
		return nil
	}

	b.RWmutex.Lock()
	b.removeLocked(id, values)
	b.RWmutex.Unlock()

	return nil
}

func (b *IndexBtree) Add(id int64, data []byte) error {
	values, indexed, missing := b.values(data)
	if !indexed {
		if b.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field '%s' not defined", missing)
	}

	item := &RowOrdered{ID: id, Values: values}

	b.RWmutex.Lock()
	defer b.RWmutex.Unlock()

	if b.Options.Unique {
		if found, ok := b.Btree.Get(item); ok && found.ID != id {
			return fmt.Errorf("key already exists for unique btree index")
		}
	}
	if old, replaced := b.Btree.ReplaceOrInsert(item); replaced {
		b.bytes -= rowOrderedSize(old)
	}
	b.bytes += rowOrderedSize(item)

	return nil
}

// Release removes the entry of oldData owned by id when newData uses a
// different key.
func (b *IndexBtree) Release(id int64, oldData, newData []byte) {
	oldValues, oldIndexed, _ := b.values(oldData)
	if !oldIndexed {
		return
	}
	newValues, newIndexed, _ := b.values(newData)
	if newIndexed && compareTupleValues(b.Options, oldValues, newValues) == 0 {
		return
	}

	b.RWmutex.Lock()
	b.removeLocked(id, oldValues)
	b.RWmutex.Unlock()
}

type IndexBtreeTraverse struct {
	Reverse       bool                   `json:"reverse"`
	From          map[string]interface{} `json:"from"`
	To            map[string]interface{} `json:"to"`
	FromExclusive map[string]interface{} `json:"from>"`
	ToExclusive   map[string]interface{} `json:"to<"`
}

func (b *IndexBtree) buildBound(bound map[string]interface{}, lower bool) *RowOrdered {
	values := make([]interface{}, 0, len(b.Options.Fields))
	for _, field := range b.Options.Fields {
		cleanField := strings.TrimPrefix(field, "-")
		if val, exists := bound[cleanField]; exists {
			values = append(values, val)
			continue
		}

		if lower {
			values = append(values, boundLow)
		} else {
			values = append(values, boundHigh)
		}
	}

	id := int64(math.MinInt64)
	if !lower {
		id = math.MaxInt64
	}
	return &RowOrdered{ID: id, Values: values}
}

func (b *IndexBtree) Traverse(optionsData []byte, f func(id int64, data []byte) bool) {
	options := &IndexBtreeTraverse{}
	_ = json.Unmarshal(optionsData, options)

	lowerBound := options.From
	lowerExclusive := false
	if len(options.FromExclusive) > 0 {
		lowerBound = options.FromExclusive
		lowerExclusive = true
	}

	upperBound := options.To
	upperExclusive := false
	if len(options.ToExclusive) > 0 {
		upperBound = options.ToExclusive
		upperExclusive = true
	}

	hasLower := len(lowerBound) > 0
	hasUpper := len(upperBound) > 0

	var pivotLower *RowOrdered
	if hasLower {
		pivotLower = b.buildBound(lowerBound, true)
	}

	var pivotUpper *RowOrdered
	if hasUpper {
		pivotUpper = b.buildBound(upperBound, false)
	}

	iterator := func(r *RowOrdered) bool {
		if !options.Reverse {
			if hasLower {
				cmp := compareTupleValues(b.Options, r.Values, pivotLower.Values)
				if cmp < 0 || (cmp == 0 && lowerExclusive) {
					return true
				}
			}

			if hasUpper {
				cmp := compareTupleValues(b.Options, r.Values, pivotUpper.Values)
				if cmp > 0 || (cmp == 0 && upperExclusive) {
					return false
				}
			}
		} else {
			if hasUpper {
				cmp := compareTupleValues(b.Options, r.Values, pivotUpper.Values)
				if cmp > 0 || (cmp == 0 && upperExclusive) {
					return true
				}
			}

			if hasLower {
				cmp := compareTupleValues(b.Options, r.Values, pivotLower.Values)
				if cmp < 0 || (cmp == 0 && lowerExclusive) {
					return false
				}
			}
		}

		return f(r.ID, nil)
	}

	b.RWmutex.RLock()
	defer b.RWmutex.RUnlock()

	if options.Reverse {
		if hasUpper {
			b.Btree.DescendLessOrEqual(pivotUpper, iterator)
		} else {
			b.Btree.Descend(iterator)
		}
		return
	}

	if hasLower {
		b.Btree.AscendGreaterOrEqual(pivotLower, iterator)
	} else {
		b.Btree.Ascend(iterator)
	}
}

func (b *IndexBtree) GetType() string {
	return "btree"
}

func (b *IndexBtree) GetOptions() interface{} {
	return b.Options
}

func (b *IndexBtree) IsUnique() bool {
	return b.Options.Unique
}

func (b *IndexBtree) Size() int64 {
	b.RWmutex.RLock()
	defer b.RWmutex.RUnlock()
	return b.bytes
}

// --- IndexFTS ---

type IndexFTS struct {
	Index   map[string]map[int64]struct{}
	RWmutex *sync.RWMutex
	Options *IndexFTSOptions

	bytes int64 // guarded by RWmutex
}

// indexFTSPostingSize estimates the memory of an id in the set of a token.
const indexFTSPostingSize = 16

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
	field := i.Options.Field
	val, dt, err := simdscan.GetField(data, field)
	if err != nil {
		return nil // Field missing, skip
	}
	if dt != simdscan.TypeString {
		return nil // Not a string, skip
	}

	tokens := i.tokenize(string(val))

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	for _, token := range tokens {
		rows, ok := i.Index[token]
		if !ok {
			rows = map[int64]struct{}{}
			i.Index[token] = rows
			i.bytes += int64(len(token)) + indexEntryOverhead
		}
		if _, ok := rows[id]; !ok {
			rows[id] = struct{}{}
			i.bytes += indexFTSPostingSize
		}
	}

	return nil
}

type IndexFTSTraverse struct {
	Match string `json:"match"`
}

func (i *IndexFTS) Traverse(optionsData []byte, f func(id int64, data []byte) bool) {
	options := &IndexFTSTraverse{}
	_ = json.Unmarshal(optionsData, options)

	tokens := i.tokenize(options.Match)
	if len(tokens) == 0 {
		return
	}

	i.RWmutex.RLock()
	defer i.RWmutex.RUnlock()

	firstToken := tokens[0]
	rows, ok := i.Index[firstToken]
	if !ok {
		return
	}

	for id := range rows {
		matchAll := true
		for _, token := range tokens[1:] {
			otherRows, ok := i.Index[token]
			if !ok {
				matchAll = false
				break
			}
			if _, exists := otherRows[id]; !exists {
				matchAll = false
				break
			}
		}

		if matchAll {
			if !f(id, nil) {
				return
			}
		}
	}
}

func (i *IndexFTS) Remove(id int64, data []byte) error {
	field := i.Options.Field
	val, dt, err := simdscan.GetField(data, field)
	if err != nil {
		return nil
	}
	if dt != simdscan.TypeString {
		return nil
	}

	tokens := i.tokenize(string(val))

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	for _, token := range tokens {
		if rows, ok := i.Index[token]; ok {
			if _, ok := rows[id]; ok {
				delete(rows, id)
				i.bytes -= indexFTSPostingSize
			}
			if len(rows) == 0 {
				delete(i.Index, token)
				i.bytes -= int64(len(token)) + indexEntryOverhead
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

func (i *IndexFTS) IsUnique() bool {
	return false
}

func (i *IndexFTS) Size() int64 {
	i.RWmutex.RLock()
	defer i.RWmutex.RUnlock()
	return i.bytes
}
