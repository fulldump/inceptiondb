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

// how to try with curl:
// start with tls: HTTPSENABLED=TRUE HTTPSSELFSIGNED=TRUE make run
// curl -v -X POST -T. -k https://localhost:8080/v1/collections/prueba:insert
// type one document and press enter
func insertStream(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		collection, err = s.CreateCollection(collectionName)
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	jsonWriter := json.NewEncoder(w)
	jsonReader := json.NewDecoder(r.Body)

	//w.WriteHeader(http.StatusCreated)

	for {
		item := map[string]interface{}{}
		err := jsonReader.Decode(&item)
		if err == io.EOF {
			// w.WriteHeader(http.StatusCreated)
			return nil
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			// w.WriteHeader(http.StatusBadRequest)
			return err
		}
		_, err = collection.Insert(item)
		if err != nil {
			// TODO: handle error properly
			// w.WriteHeader(http.StatusConflict)
			return err
		}

		jsonWriter.Encode(item)
	}

}
