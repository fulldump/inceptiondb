package apicollectionv1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SierraSoftworks/connor"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/utils"
)

func traverse(requestBody []byte, col *collection.Collection, f func(id int64, payload []byte) bool) error {

	options := &struct {
		Index  *string
		Filter map[string]interface{}
		Skip   int64
		Limit  int64
	}{
		Index:  nil,
		Filter: nil,
		Skip:   0,
		Limit:  1,
	}
	err := json.Unmarshal(requestBody, &options)
	if err != nil {
		return err
	}

	hasFilter := len(options.Filter) > 0

	// Add simple equality filter check
	isSimple := true
	if hasFilter {
		for k, v := range options.Filter {
			if strings.HasPrefix(k, "$") || strings.Contains(k, ".") {
				isSimple = false
				break
			}
			switch v.(type) {
			case string, float64, bool, nil:
				// supported
			default:
				isSimple = false
				break
			}
		}
	}

	var simpleFilter equalityFilter
	if hasFilter && isSimple {
		simpleFilter = compileEqualityFilter(options.Filter)
	}

	skip := options.Skip
	limit := options.Limit
	iterator := func(id int64, payload []byte) bool {
		if limit == 0 {
			return false
		}

		if hasFilter {
			// Fast path for simple equality queries
			if isSimple {
				if !simpleFilter.match(payload) {
					return true
				}
				goto evaluate
			}

			// Slow path via json.Unmarshal and connor
			rowData := map[string]interface{}{}
			json.Unmarshal(payload, &rowData) // todo: handle error here?

			match, err := connor.Match(options.Filter, rowData)
			if err != nil {
				// todo: handle error?
				// return fmt.Errorf("match: %w", err)
				return false
			}
			if !match {
				return true
			}
		}

	evaluate:
		if skip > 0 {
			skip--
			return true
		}
		limit--
		return f(id, payload)
	}

	// Fullscan
	if options.Index == nil {
		traverseFullscan(col, iterator)
		return nil
	}

	indexes := col.ListIndexes()
	index, exists := indexes[*options.Index]
	if !exists {
		return fmt.Errorf("index '%s' not found, available indexes %v", *options.Index, utils.GetKeys(indexes))
	}

	_ = index
	return col.TraverseIndex(*options.Index, requestBody, iterator)
}

func traverseFullscan(col *collection.Collection, f func(id int64, payload []byte) bool) error {
	col.TraverseRecords(func(id int64, payload []byte) bool {
		return f(id, payload)
	})

	return nil
}
