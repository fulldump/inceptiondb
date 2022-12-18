package apicollectionv1

import (
	"context"
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
	}
	if err != nil {
		return err // todo: handle/wrap this properly
	}

	err = col.DropIndex(input.Name)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	w.WriteHeader(http.StatusNoContent)

	return nil
}
