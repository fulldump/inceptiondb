package apicollectionv1

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/fulldump/box"
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

	var result error

	traverse(requestBody, col, func(id int64, payload []byte) bool {
		waitParam := r.URL.Query().Get("wait") == "true"
		err := col.Delete(id, waitParam)
		if err != nil {
			result = err
			return false
		}

		w.Write(payload)
		w.Write([]byte("\n"))
		return true
	})

	return result
}
