package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fulldump/inceptiondb/collection"
)

func insertItem(collections map[string]*collection.Collection) interface{} {

	type Item map[string]interface{}

	return func(ctx context.Context, w http.ResponseWriter, r *http.Request) {

		collectionName := getParam(ctx, "collection_name")
		collection := collections[collectionName]

		jsonReader := json.NewDecoder(r.Body)
		// jsonWriter := json.NewEncoder(w)

		for {
			item := map[string]interface{}{}
			err := jsonReader.Decode(&item)
			if err != nil {
				// TODO: handle error properly
				fmt.Println("ERROR:", err.Error())
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, err = collection.Insert(item)
			if err != nil {
				// TODO: handle error properly
				fmt.Println("ERROR:", err.Error())
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			// jsonWriter.Encode(item)
		}

	}
}
