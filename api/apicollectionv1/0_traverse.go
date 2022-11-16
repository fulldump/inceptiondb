package apicollectionv1

import (
	"encoding/json"
	"fmt"

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
