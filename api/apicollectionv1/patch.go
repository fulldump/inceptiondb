package apicollectionv1

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"net/http"

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

	wb := bufio.NewWriterSize(w, 64*1024)
	defer wb.Flush()

	traverse(requestBody, col, func(id int64, payload []byte) bool {

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

		wb.Write(updated)
		wb.Write([]byte("\n"))

		return true
	})

	return nil
}
