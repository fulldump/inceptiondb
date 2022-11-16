package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func find(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	input := struct {
		Index *string
	}{}
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

	if input.Index == nil {
		traverseFullscan(requestBody, col, func(row *collection.Row) {
			writeRow(w, row)
		})
		return nil
	}

	// todo: if index not found: return error
	index, exists := col.Indexes[*input.Index]
	if !exists {
		return fmt.Errorf("index '%s' not found, available indexes %v", *input.Index, GetKeys(col.Indexes))
	}

	index.Traverse(requestBody, func(row *collection.Row) bool {
		writeRow(w, row)
		return true
	})

	return nil
}

func writeRow(w http.ResponseWriter, row *collection.Row) {
	w.Write(row.Payload)
	w.Write([]byte("\n"))
}
