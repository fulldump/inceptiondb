package api

import (
	"context"
	"net/http"

	"inceptiondb/collection"
)

func listItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context, w http.ResponseWriter) {

		collectionName := getParam(ctx, "collection_name")
		collections[collectionName].Traverse(func(data []byte) {
			w.Write(data)
			w.Write([]byte("\n"))
		})

	}
}
