package apicollectionv1

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"io"
	"net/http"

	"github.com/SierraSoftworks/connor"
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
		Filter map[string]interface{}
		Patch  interface{}
	}{}
	jsonv2.Unmarshal(requestBody, &patch) // TODO: handle err

	e := json.NewEncoder(w)

	traverse(requestBody, col, func(row *collection.Row) bool {

		row.PatchMutex.Lock()
		defer row.PatchMutex.Unlock()

		hasFilter := patch.Filter != nil && len(patch.Filter) > 0
		if hasFilter {

			rowData := map[string]interface{}{}
			jsonv2.Unmarshal(row.Payload, &rowData) // todo: handle error here?

			match, err := connor.Match(patch.Filter, rowData)
			if err != nil {
				// todo: handle error?
				// return fmt.Errorf("match: %w", err)
				return false
			}
			if !match {
				return false
			}
		}

		err := col.Patch(row, patch.Patch)
		if err != nil {
			// TODO: handle err??
			// return err
			return true // todo: OR return false?
		}

		e.Encode(row.Payload) // todo: handle err?

		return true
	})

	return nil
}
