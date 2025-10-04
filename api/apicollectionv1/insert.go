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

	wc := http.NewResponseController(w)
	wcerr := wc.EnableFullDuplex()
	if wcerr != nil {
		return wcerr
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		collection, err = s.CreateCollection(collectionName)
		if err != nil {
			return err // todo: handle/wrap this properly
		}
		err = collection.SetDefaults(newCollectionDefaults())
		if err != nil {
			return err // todo: handle/wrap this properly
		}
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	// READER

	// ALT 1
	jsonReader := json.NewDecoder(r.Body)

	// ALT 2
	// jsonReader := jsontext.NewDecoder(r.Body, jsontext.AllowDuplicateNames(true))

	// WRITER

	// ALT 1
	// jsonWriter := json.NewEncoder(w)

	// ALT 2
	// jsonWriter := jsontext.NewEncoder(w)

	// ALT 3
	// not needed

	// item := map[string]any{} // Idea: same item and clean on each iteration
	for i := 0; true; i++ {
		item := map[string]any{}
		// READER:ALT 1
		err := jsonReader.Decode(&item)
		// READER:ALT 2
		// err := json2.UnmarshalDecode(jsonReader, &item)
		if err == io.EOF {
			if i == 0 {
				w.WriteHeader(http.StatusNoContent)
			}
			return nil
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			if i == 0 {
				w.WriteHeader(http.StatusBadRequest)
			}
			return err
		}
		row, err := collection.Insert(item)
		if err != nil {
			// TODO: handle error properly
			if i == 0 {
				w.WriteHeader(http.StatusConflict)
			}
			return err
		}

		if i == 0 {
			w.WriteHeader(http.StatusCreated)
		}

		// ALT 1
		// jsonWriter.Encode(row.Payload)

		// ALT 2
		// json2.MarshalEncode(jsonWriter, row.Payload,
		// jsontext.AllowDuplicateNames(true),
		// jsontext.AllowInvalidUTF8(true),
		// )

		// ALT 3
		fmt.Fprintln(w, string(row.Payload))

		// ALT 4
		// query param to optionally write nothing

		// for k := range item {
		// 	delete(item, k)
		// }
	}

	return nil
}
