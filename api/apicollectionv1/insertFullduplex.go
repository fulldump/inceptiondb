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

func insertFullduplex(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	wc := http.NewResponseController(w)
	wcerr := wc.EnableFullDuplex()
	if wcerr != nil {
		fmt.Println("ERRRRRR", wcerr.Error())
	}

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
	jsonWriter := json.NewEncoder(w)

	flusher, ok := w.(http.Flusher)
	if ok {
		fmt.Println("FLUSHER!")
	} else {
		fmt.Println("NO FLUSHER")
	}

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
			w.WriteHeader(http.StatusConflict)
			return err
		}

		// fmt.Println("item inserted")
		if ok {
			flusher.Flush()
		}

		err = jsonWriter.Encode(item)
		if err != nil {
			fmt.Println("ERROR:", err.Error())
		}
	}

}
