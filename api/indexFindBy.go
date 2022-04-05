package api

import (
	"context"
	"fmt"

	"inceptiondb/collection"
)

func indexFindBy(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context) (interface{}, error) {

		collectionName := getParam(ctx, "collection_name")
		indexName := getParam(ctx, "index_name")
		value := getParam(ctx, "value")
		result := map[string]interface{}{}
		err := collections[collectionName].FindBy(indexName, value, &result)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		return result, nil
	}
}