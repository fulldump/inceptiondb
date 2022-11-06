package apicollectionv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func patch(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

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

	e := json.NewEncoder(w)

	index, exists := col.Indexes[input.Index]
	if !exists {
		traverseFullscan(rquestBody, col, func(row *collection.Row) {
			patchRow(rquestBody, col, row, e)
		})
		return nil
	}

	index.Traverse(rquestBody, func(row *collection.Row) bool {
		patchRow(rquestBody, col, row, e)
		return true
	})

	return nil
}

func patchRow(input []byte, col *collection.Collection, row *collection.Row, e *json.Encoder) {
	patch := struct {
		Patch interface{}
	}{}
	json.Unmarshal(input, &patch) // TODO: handle err

	_ = col.Patch(row, patch.Patch) // TODO: handle err
	e.Encode(row.Payload)
}
