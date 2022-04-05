package api

import (
	"context"

	"inceptiondb/collection"
)

func insertItem(collections map[string]*collection.Collection) interface{} {

	type Item map[string]interface{}

	return func(ctx context.Context, item Item) (Item, error) {

		collectionName := getParam(ctx, "collection_name")

		err := collections[collectionName].Insert(item)
		if err != nil {
			return nil, err
		}

		return item, nil
	}
}
