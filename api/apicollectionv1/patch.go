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

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	patch := struct {
		Patch interface{}
	}{}
	json.Unmarshal(requestBody, &patch) // TODO: handle err

	e := json.NewEncoder(w)

	traverse(requestBody, col, func(row *collection.Row) bool {
		err := col.Patch(row, patch.Patch)

		if err != nil {
			// TODO: handle err??
			// return err
			return true
		}

		e.Encode(row.Payload) // todo: handle err?

		return true
	})

	return nil
}
