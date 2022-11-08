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

	i := int64(0)
	from := params.Skip
	to := params.Skip + params.Limit
	for _, row := range col.Rows {

		rowData := map[string]interface{}{}
		json.Unmarshal(row.Payload, &rowData)

		if match, err := connor.Match(params.Filter, rowData); err != nil {
			// TODO: wrap with http error: w.WriteHeader(http.StatusBadRequest)
			return err
		} else if match {
			if i < from {
				i++
				continue
			}
			if to == 0 || i < to {
				i++
				f(row)
				continue
			}
			break
		} else {
			continue
		}

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
