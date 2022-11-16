package apicollectionv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func remove(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	input := struct {
		Index string
	}{
		Index: "",
	}
	err = json.Unmarshal(requestBody, &input)
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
		traverseFullscan(requestBody, col, func(row *collection.Row) {
			removeRow(col, row, w)
		})
		return nil
	}

	index.Traverse(requestBody, func(row *collection.Row) bool {
		removeRow(col, row, w)
		return true
	})

	return nil
}

func removeRow(col *collection.Collection, row *collection.Row, w http.ResponseWriter) {
	err := col.Remove(row)
	if err != nil {
		return
	}

	w.Write(row.Payload)
	w.Write([]byte("\n"))
}
