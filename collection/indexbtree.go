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
}

type RowOrdered struct {
	*Row
	Values []interface{}
}

type IndexBTreeOptions struct {
	Fields []string
	Sparse bool
	Unique bool
}

// todo: not used?
type IndexBtreeOptions struct {
	Name string `json:"name"`
	*IndexBTreeOptions
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

type TraverseOptions struct {
	Reverse bool
}

func (b *IndexBtree) Traverse(optionsData []byte, f func(*Row) bool) {

	options := &IndexBtreeTraverse{}
	json.Unmarshal(optionsData, options) // todo: handle error

	if options.Reverse {
		b.Btree.Descend(func(r *RowOrdered) bool {
			return f(r.Row)
		})
		return
	}

	b.Btree.Ascend(func(r *RowOrdered) bool {
		return f(r.Row)
	})
}
