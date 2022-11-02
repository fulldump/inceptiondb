package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

func insert(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		collection, err = s.CreateCollection(collectionName)
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	jsonReader := json.NewDecoder(r.Body)

	for {
		item := map[string]interface{}{}
		err := jsonReader.Decode(&item)
		if err == io.EOF {
			w.WriteHeader(http.StatusCreated)
			return nil
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			w.WriteHeader(http.StatusBadRequest)
			return err
		}
		_, err = collection.Insert(item)
		if err != nil {
			// TODO: handle error properly
			w.WriteHeader(http.StatusConflict)
			return err
		}

		// jsonWriter.Encode(item)
	}

}
