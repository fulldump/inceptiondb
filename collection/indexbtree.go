package collection

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/google/btree"
)

type IndexBtree struct {
	Btree   *btree.BTreeG[*RowOrdered]
	Options *IndexBTreeOptions
}

func (b *IndexBtree) RemoveRow(r *Row) error {

	// TODO: duplicated code:
	values := []interface{}{}
	data := map[string]interface{}{}
	json.Unmarshal(r.Payload, &data)

	for _, field := range b.Options.Fields {
		values = append(values, data[field])
	}

	b.Btree.Delete(&RowOrdered{
		Row:    r, // probably r is not needed
		Values: values,
	})

	return nil
}

type IndexBtreeTraverse struct {
	Reverse bool                   `json:"reverse"`
	Limit   int64                  `json:"limit"`
	Skip    int64                  `json:"skip"`
	From    map[string]interface{} `json:"from"`
	To      map[string]interface{} `json:"to"`
}

type RowOrdered struct {
	*Row
	Values []interface{}
}

type IndexBTreeOptions struct {
	Fields []string `json:"fields"`
	Sparse bool     `json:"sparse"`
	Unique bool     `json:"unique"`
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
			field = strings.TrimPrefix(field, "-")

			switch valA := valA.(type) {
			case string:
				valB, ok := valB.(string)
				if !ok {
					panic("Type B should be string for field " + field)
				}
				if reverse {
					return !(valA < valB)
				} else {
					return valA < valB
				}

			case float64:
				valB, ok := valB.(float64)
				if !ok {
					panic("Type B should be float64 for field " + field)
				}
				if reverse {
					return !(valA < valB)
				} else {
					return valA < valB
				}

				// todo: case bool
			default:
				panic("Type A not supported, field " + field)
			}
		}

		return false
	})

	return &IndexBtree{
		Btree:   index,
		Options: options,
	}
}

func (b *IndexBtree) AddRow(r *Row) error {
	values := []interface{}{}
	data := map[string]interface{}{}
	json.Unmarshal(r.Payload, &data)

	for _, field := range b.Options.Fields {
		field = strings.TrimPrefix(field, "-")
		values = append(values, data[field])
	}

	if b.Btree.Has(&RowOrdered{Values: values}) {
		return fmt.Errorf("already exists")
	}

	b.Btree.ReplaceOrInsert(&RowOrdered{
		Row:    r,
		Values: values,
	})

	return nil
}

func (b *IndexBtree) Traverse(optionsData []byte, f func(*Row) bool) {

	options := &IndexBtreeTraverse{
		Limit: 1,
	}
	json.Unmarshal(optionsData, options) // todo: handle error

	skip := options.Skip
	limit := options.Limit
	iterator := func(r *RowOrdered) bool {
		if skip > 0 {
			skip--
			return true
		}
		if limit == 0 {
			return false
		}
		limit--
		return f(r.Row)
	}

	hasFrom := len(options.From) > 0
	hasTo := len(options.To) > 0

	pivotFrom := &RowOrdered{}
	if hasFrom {
		for _, field := range b.Options.Fields {
			pivotFrom.Values = append(pivotFrom.Values, options.From[field])
		}
	}

	pivotTo := &RowOrdered{}
	if hasFrom {
		for _, field := range b.Options.Fields {
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
