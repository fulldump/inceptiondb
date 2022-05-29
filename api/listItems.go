package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/SierraSoftworks/connor"

	"inceptiondb/collection"
)

func listItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

		collectionName := getParam(ctx, "collection_name")

		from := 0
		from, _ = strconv.Atoi(r.URL.Query().Get("skip"))

		limit := 0
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))

		filter := r.URL.Query().Get("filter")

		if filter == "" {
			collections[collectionName].TraverseRange(from, from+limit, func(row *collection.Row) {
				w.Write(row.Payload)
				w.Write([]byte("\n"))
			})
			return nil
		}

		fmt.Println("DEBUG: filter=", filter)

		filterParsed := map[string]interface{}{}
		filterErr := json.Unmarshal([]byte(filter), &filterParsed)
		if filterErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			return filterErr
		}

		c := collections[collectionName]
		i := 0
		to := from + limit
		for _, row := range c.Rows {

			rowData := map[string]interface{}{}
			json.Unmarshal(row.Payload, &rowData)

			if match, err := connor.Match(filterParsed, rowData); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return err
			} else if match {
				if i < from {
					i++
					continue
				}
				if to == 0 || i < to {
					i++
					w.Write(row.Payload)
					w.Write([]byte("\n"))
					continue
				}
				break
			} else {
				continue
			}

		}

		return nil
	}
}
