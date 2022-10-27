package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func patch(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

	rquestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}

	input := struct {
		Mode string
	}{
		Mode: "fullscan",
	}
	err = json.Unmarshal(rquestBody, &input)
	if err != nil {
		return err
	}

	f, exist := patchModes[input.Mode]
	if !exist {
		box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("bad mode '%s', must be [%s]. See docs: TODO", input.Mode, strings.Join(GetKeys(patchModes), "|"))
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	return f(rquestBody, collection, w)
}

var patchModes = map[string]func(input []byte, col *collection.Collection, w http.ResponseWriter) error{
	"fullscan": func(input []byte, col *collection.Collection, w http.ResponseWriter) error {
		return traverseFullscan(input, col, patchRow(input, col, json.NewEncoder(w)))
	},
	"unique": func(input []byte, col *collection.Collection, w http.ResponseWriter) (err error) {
		return traverseUnique(input, col, patchRow(input, col, json.NewEncoder(w)))
	},
}

func patchRow(input []byte, col *collection.Collection, e *json.Encoder) func(row *collection.Row) {
	return func(row *collection.Row) {
		patch := struct {
			Patch interface{}
		}{}
		json.Unmarshal(input, &patch) // TODO: handle err

		_ = col.Patch(row, patch.Patch) // TODO: handle err
		e.Encode(row.Payload)
	}
}
