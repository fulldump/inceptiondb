package api

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fulldump/inceptiondb/collection"
)

func deleteCollection(collections map[string]*collection.Collection) interface{} {
	return func(ctx context.Context, w http.ResponseWriter) error {

		collectionName := getParam(ctx, "collection_name")

		collection, found := collections[collectionName]
		if !found {
			return fmt.Errorf("collection '%s' not found", collectionName)
		}

		err := collection.Drop()
		if err != nil {
			return err
		}

		delete(collections, collectionName)

		return nil
	}
}
