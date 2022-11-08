package apicollectionv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func find(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	rquestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	input := struct {
		Index string
	}{
		Index: "",
	}
	err = json.Unmarshal(rquestBody, &input)
	if err != nil {
		return err
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	index, exists := col.Indexes[input.Index]
	if !exists {
		traverseFullscan(rquestBody, col, func(row *collection.Row) {
			w.Write(row.Payload)
			w.Write([]byte("\n"))
		})
		return nil
	}

	index.Traverse(rquestBody, func(row *collection.Row) bool {
		w.Write(row.Payload)
		w.Write([]byte("\n"))
		return true
	})

	return nil
}

// TODO: remove this
var findModes = map[string]func(input []byte, col *collection.Collection, w http.ResponseWriter) error{
	"fullscan": func(input []byte, col *collection.Collection, w http.ResponseWriter) error {
		return traverseFullscan(input, col, writeRow(w))
	},
	"unique": func(input []byte, col *collection.Collection, w http.ResponseWriter) error {
		return traverseUnique(input, col, writeRow(w))
	},
}

func writeRow(w http.ResponseWriter) func(r *collection.Row) {
	return func(row *collection.Row) {
		w.Write(row.Payload)
		w.Write([]byte("\n"))
	}
}
