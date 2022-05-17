package api

import (
	"context"
	"net/http"

	"inceptiondb/collection"
)

func insertItem(collections map[string]*collection.Collection) interface{} {

	type Item map[string]interface{}

	return func(ctx context.Context, w http.ResponseWriter, item Item) (Item, error) {

		collectionName := getParam(ctx, "collection_name")

		err := collections[collectionName].Insert(item)
		if err != nil {
			w.WriteHeader(http.StatusConflict)
			return nil, err
		}

		return item, nil
	}
}
