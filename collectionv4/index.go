package collectionv4

import (
	"encoding/json"
	"fmt"
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
}

// --- IndexMap ---

type IndexMap struct {
	Entries map[string]*IndexMapEntry
	RWmutex *sync.RWMutex
	Options *IndexMapOptions
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
	field := i.Options.Field
	val, dt, err := simdscan.GetField(data, field)
	if err != nil {
		return nil
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	switch dt {
	case simdscan.TypeString:
		delete(i.Entries, string(val))
	case simdscan.TypeArray:
		jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
			if dataType == jsonparser.String {
				delete(i.Entries, string(value))
			}
		}, field)
	}

	return nil
}

func (i *IndexMap) Add(id int64, data []byte) error {
	field := i.Options.Field
	val, dt, err := simdscan.GetField(data, field)
	if err != nil {
		if i.Options.Sparse {
			return nil
		}
		return fmt.Errorf("field `%s` is indexed and mandatory", field)
	}

	i.RWmutex.Lock()
	defer i.RWmutex.Unlock()

	switch dt {
	case simdscan.TypeString:
		k := string(val)
		if _, exists := i.Entries[k]; exists {
			return fmt.Errorf("index conflict: field '%s' with value '%s'", field, k)
		}
		i.Entries[k] = &IndexMapEntry{ID: id}

	case simdscan.TypeArray:
		// First pass: check for conflicts
		var conflict error
		jsonparser.ArrayEach(data, func(value []byte, adt jsonparser.ValueType, offset int, err error) {
			if conflict != nil {
				return
			}
			if adt == jsonparser.String {
				s := string(value)
				if _, exists := i.Entries[s]; exists {
					conflict = fmt.Errorf("index conflict: field '%s' with value '%s'", field, s)
				}
			}
		}, field)
		if conflict != nil {
			return conflict
		}
		// Second pass: insert all
		jsonparser.ArrayEach(data, func(value []byte, adt jsonparser.ValueType, offset int, err error) {
			if adt == jsonparser.String {
				i.Entries[string(value)] = &IndexMapEntry{ID: id}
			}
		}, field)

	default:
		return fmt.Errorf("type not supported by IndexMap")
	}

	return nil
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

// --- IndexBtree ---

type IndexBTreeOptions struct {
	Fields []string `json:"fields"`
	Sparse bool     `json:"sparse"`
	Unique bool     `json:"unique"`
}

type IndexBtree struct {
	Btree   *btree.BTreeG[*RowOrdered]
	RWmutex *sync.RWMutex
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
		return compareTupleValues(options, a.Values, b.Values) < 0
	})

	return &IndexBtree{
		Btree:   index,
		RWmutex: &sync.RWMutex{},
		Options: options,
	}
}

func (b *IndexBtree) Remove(id int64, data []byte) error {
	values := make([]interface{}, 0, len(b.Options.Fields))
	for _, field := range b.Options.Fields {
		cleanField := strings.TrimPrefix(field, "-")
		val, dt, err := simdscan.GetField(data, cleanField)
		if err != nil {
			continue
		}
		values = append(values, simdToInterface(val, dt))
	}

	b.RWmutex.Lock()
	b.Btree.Delete(&RowOrdered{
		ID:     id,
		Values: values,
	})
	b.RWmutex.Unlock()

	return nil
}

func (b *IndexBtree) Add(id int64, data []byte) error {
	var values []interface{}
	for _, field := range b.Options.Fields {
		cleanField := strings.TrimPrefix(field, "-")
		val, dt, err := simdscan.GetField(data, cleanField)
		if err != nil {
			if b.Options.Sparse {
				return nil
			}
			return fmt.Errorf("field '%s' not defined", cleanField)
		}
		values = append(values, simdToInterface(val, dt))
	}

	if b.Options.Unique {
		b.RWmutex.RLock()
		if b.Btree.Has(&RowOrdered{Values: values}) {
			b.RWmutex.RUnlock()
			return fmt.Errorf("key already exists for unique btree index")
		}
		b.RWmutex.RUnlock()
	}

	b.RWmutex.Lock()
	b.Btree.ReplaceOrInsert(&RowOrdered{
		ID:     id,
		Values: values,
	})
	b.RWmutex.Unlock()

	return nil
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

	return &RowOrdered{Values: values}
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
		if _, ok := i.Index[token]; !ok {
			i.Index[token] = map[int64]struct{}{}
		}
		i.Index[token][id] = struct{}{}
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

func (i *IndexFTS) IsUnique() bool {
	return false
}
