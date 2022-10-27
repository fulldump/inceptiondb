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

func remove(ctx context.Context, w http.ResponseWriter, r *http.Request) error {

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

	f, exist := removeModes[input.Mode]
	if !exist {
		box.GetResponse(ctx).WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("bad mode '%s', must be [%s]. See docs: TODO", input.Mode, strings.Join(GetKeys(removeModes), "|"))
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	return f(rquestBody, collection, w)
}

var removeModes = map[string]func(input []byte, col *collection.Collection, w http.ResponseWriter) error{
	"fullscan": func(input []byte, col *collection.Collection, w http.ResponseWriter) error {
		traverseFullscan(input, col, removeRow(col, w))
		return nil
	},
	"unique": func(input []byte, col *collection.Collection, w http.ResponseWriter) (err error) {
		return traverseUnique(input, col, removeRow(col, w))
	},
}

func removeRow(col *collection.Collection, w http.ResponseWriter) func(row *collection.Row) {
	return func(row *collection.Row) {

		err := col.Remove(row)
		if err != nil {
			return
		}

		w.Write(row.Payload)
		w.Write([]byte("\n"))
	}
}
