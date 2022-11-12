package apicollectionv1

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/SierraSoftworks/connor"

	"github.com/fulldump/inceptiondb/collection"
)

func traverseFullscan(input []byte, col *collection.Collection, f func(row *collection.Row)) error {

	params := &struct {
		Filter map[string]interface{}
		Skip   int64
		Limit  int64
	}{
		Filter: map[string]interface{}{},
		Skip:   0,
		Limit:  1,
	}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return err
	}

	hasFilter := params.Filter != nil && len(params.Filter) > 0

	skip := params.Skip
	limit := params.Limit
	for _, row := range col.Rows {

		if limit == 0 {
			break
		}

		if hasFilter {
			rowData := map[string]interface{}{}
			json.Unmarshal(row.Payload, &rowData) // todo: handle error here?

			match, err := connor.Match(params.Filter, rowData)
			if err != nil {
				return fmt.Errorf("match: %w", err)
			}
			if !match {
				continue
			}
		}

		if skip > 0 {
			skip--
			continue
		}

		limit--
		f(row)
	}

	return nil
}

func traverseUnique(input []byte, col *collection.Collection, f func(row *collection.Row)) error {

	params := &struct {
		Index string
		Value string
	}{}
	err := json.Unmarshal(input, &params)
	if err != nil {
		return err
	}

	index, exist := col.Indexes[params.Index]
	if !exist {
		return fmt.Errorf("index '%s' does not exist", params.Index)
	}

	traverseOptions, err := json.Marshal(collection.IndexMapTraverse{
		Value: params.Value,
	})
	if err != nil {
		return fmt.Errorf("marshal traverse options: %s", err.Error())
	}

	index.Traverse(traverseOptions, func(row *collection.Row) bool {
		f(row)
		return true
	})

	return nil
}

// TODO: move to package utils/diogenes
func GetKeys[T any](m map[string]T) []string {
	keys := []string{}
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
