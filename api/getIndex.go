package api

import (
	"context"
	"fmt"

	"inceptiondb/collection"
)

func getIndex(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context) (*collection.IndexOptions, error) {

		collectionName := getParam(ctx, "collection_name")
		indexName := getParam(ctx, "index_name")
		_, exists := collections[collectionName].Indexes[indexName]
		if !exists {
			return nil, fmt.Errorf("index '%s' does not exist", indexName)
		}

		return &collection.IndexOptions{
			Field: indexName,
		}, nil
	}
}
