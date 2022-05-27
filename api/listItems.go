package api

import (
	"context"
	"net/http"
	"strconv"

	"inceptiondb/collection"
)

func listItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {

		collectionName := getParam(ctx, "collection_name")

		from := 0
		from, _ = strconv.Atoi(r.URL.Query().Get("skip"))

		limit := 0
		limit, _ = strconv.Atoi(r.URL.Query().Get("limit"))

		collections[collectionName].TraverseRange(from, from+limit, func(row *collection.Row) {
			w.Write(row.Payload)
			w.Write([]byte("\n"))
		})

	}
}
