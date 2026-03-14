package apicollectionv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/SierraSoftworks/connor"
	"github.com/fulldump/box"
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
	json.Unmarshal(requestBody, &patch) // TODO: handle err

	traverse(requestBody, col, func(id int64, payload []byte) bool {

		hasFilter := len(patch.Filter) > 0
		if hasFilter {
			rowData := map[string]interface{}{}
			json.Unmarshal(payload, &rowData) // todo: handle error here?

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

		waitParam := r.URL.Query().Get("wait") == "true"
		err := col.Patch(id, patch.Patch, waitParam)
		if err != nil {
			// TODO: handle err??
			// return err
			return true // todo: OR return false?
		}

		updated, ok := col.Get(id)
		if !ok {
			return false
		}

		w.Write(updated)
		w.Write([]byte("\n"))

		return true
	})

	return nil
}
