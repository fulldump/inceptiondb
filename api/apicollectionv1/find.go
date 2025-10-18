package apicollectionv1

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
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
	err = jsonv2.Unmarshal(requestBody, &input)
	if err != nil {
		return err
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	return traverse(requestBody, col, func(row *collection.Row) bool {
		w.Write(row.Payload)
		w.Write([]byte("\n"))
		return true
	})
}
