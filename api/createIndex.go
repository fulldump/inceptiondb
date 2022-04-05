package api

import (
	"context"

	"inceptiondb/collection"
)

func createIndex(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context, indexOptions *collection.IndexOptions) (*collection.IndexOptions, error) {

		collectionName := getParam(ctx, "collection_name")
		err := collections[collectionName].Index(indexOptions)
		if err != nil {
			return nil, err
		}

		return indexOptions, nil
	}
}
