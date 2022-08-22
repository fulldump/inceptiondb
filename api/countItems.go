package api

import (
	"context"

	"github.com/fulldump/inceptiondb/collection"
)

func countItems(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context) interface{} {

		collectionName := getParam(ctx, "collection_name")

		return map[string]interface{}{
			"count": len(collections[collectionName].Rows),
		}
	}
}
