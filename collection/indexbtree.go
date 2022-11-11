package collection

import (
	"encoding/json"
	"fmt"
	"reflect"

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
	Reverse bool `json:"reverse"`
	Limit   int64
	Skip    int64
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

func NewIndexBTree(options *IndexBTreeOptions) *IndexBtree { // todo: group all arguments into a BTreeConfig struct

	index := btree.NewG(32, func(a, b *RowOrdered) bool {

		for i, valA := range a.Values {
			valB := b.Values[i]
			if reflect.DeepEqual(valA, valB) {
				continue
			}

			switch valA := valA.(type) {
			case string:
				valB, ok := valB.(string)
				if !ok {
					panic("Type B should be string for field " + options.Fields[i])
				}
				return valA < valB

			case float64:
				valB, ok := valB.(float64)
				if !ok {
					panic("Type B should be float64 for field " + options.Fields[i])
				}
				return valA < valB

				// todo: case bool
			default:
				panic("Type A not supported, field " + options.Fields[i])
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

	traverse := b.Btree.Ascend
	if options.Reverse {
		traverse = b.Btree.Descend
	}

	skip := options.Skip
	limit := options.Limit
	traverse(func(r *RowOrdered) bool {
		if skip > 0 {
			skip--
			return true
		}
		if limit == 0 {
			return false
		}
		limit--
		return f(r.Row)
	})
}
