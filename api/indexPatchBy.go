package api

import (
	"context"
	"fmt"

	"github.com/fulldump/inceptiondb/collection"
)

func indexPatchBy(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context, patch map[string]interface{}) (interface{}, error) {

		collectionName := getParam(ctx, "collection_name")
		indexName := getParam(ctx, "index_name")
		value := getParam(ctx, "value")

		col := collections[collectionName]

		row, err := col.FindByRow(indexName, value)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		err = col.Patch(row, patch)
		if err != nil {
			return nil, fmt.Errorf("could not patch item %s='%s'", indexName, value)
		}

		result := map[string]interface{}{}
		err = collections[collectionName].FindBy(indexName, value, &result)
		if err != nil {
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		return result, nil
	}
}
