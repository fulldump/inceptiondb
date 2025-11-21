package apicollectionv1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

type dropIndexRequest struct {
	Name string `json:"name"`
}

func dropIndex(ctx context.Context, w http.ResponseWriter, input *dropIndexRequest) error {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		col, err = s.CreateCollection(collectionName)
		if err != nil {
			return err // todo: handle/wrap this properly
		}
		err = col.SetDefaults(newCollectionDefaults())
		if err != nil {
			return err // todo: handle/wrap this properly
		}
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	_, exists := col.Indexes[input.Name]
	if !exists {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("index '%s' not found", input.Name)
	}

	delete(col.Indexes, input.Name)

	w.WriteHeader(http.StatusNoContent)

	return nil
}
