package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fulldump/inceptiondb/collection"
)

func indexFindBy(collections map[string]*collection.Collection) interface{} {

	return func(ctx context.Context, w http.ResponseWriter) (interface{}, error) {

		collectionName := getParam(ctx, "collection_name")
		indexName := getParam(ctx, "index_name")
		value := getParam(ctx, "value")
		result := map[string]interface{}{}
		err := collections[collectionName].FindBy(indexName, value, &result)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return nil, fmt.Errorf("item %s='%s' does not exist", indexName, value)
		}

		return result, nil
	}
}
