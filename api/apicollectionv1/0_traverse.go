package apicollectionv1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SierraSoftworks/connor"
	"github.com/buger/jsonparser"

	"github.com/fulldump/inceptiondb/collectionv4"
	"github.com/fulldump/inceptiondb/utils"
)

func traverse(requestBody []byte, col *collectionv4.Collection, f func(id int64, payload []byte) bool) error {

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

	skip := options.Skip
	limit := options.Limit
	iterator := func(id int64, payload []byte) bool {
		if limit == 0 {
			return false
		}

		if hasFilter {
			// Fast path for simple equality queries
			if isSimple {
				match := true
				for k, expected := range options.Filter {
					val, dataType, _, err := jsonparser.Get(payload, k)
					if err != nil {
						if expected != nil {
							match = false
							break
						}
						continue
					}

					switch exp := expected.(type) {
					case string:
						if dataType != jsonparser.String {
							match = false
							break
						}
						parsedStr, err := jsonparser.ParseString(val)
						if err != nil || parsedStr != exp {
							match = false
						}
					case float64:
						if dataType != jsonparser.Number {
							match = false
							break
						}
						parsedNum, err := jsonparser.ParseFloat(val)
						if err != nil || parsedNum != exp {
							match = false
						}
					case bool:
						if dataType != jsonparser.Boolean {
							match = false
							break
						}
						parsedBool, err := jsonparser.ParseBoolean(val)
						if err != nil || parsedBool != exp {
							match = false
						}
					case nil:
						if dataType != jsonparser.Null {
							match = false
						}
					default:
						match = false
					}

					if !match {
						break
					}
				}

				if !match {
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

func traverseFullscan(col *collectionv4.Collection, f func(id int64, payload []byte) bool) error {
	col.TraverseRecords(func(id int64, payload []byte) bool {
		return f(id, payload)
	})

	return nil
}
