package api

import (
	"context"

	"inceptiondb/collection"
)

func listIndexes(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context) []string {

		result := []string{}

		collectionName := getParam(ctx, "collection_name")
		for name, _ := range collections[collectionName].Indexes {
			result = append(result, name)
		}

		return result
	}
}
