package api

import (
	"context"
	"fmt"

	"github.com/fulldump/inceptiondb/collection"
)

func indexDeleteBy(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context) (interface{}, error) {

		collectionName := getParam(ctx, "collection_name")
		indexName := getParam(ctx, "index_name")
		value := getParam(ctx, "value")
		result := map[string]interface{}{}

		row, err := collections[collectionName].FindByRow(indexName, value)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		err = collections[collectionName].Remove(row)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		return result, nil
	}
}
