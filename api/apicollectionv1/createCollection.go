package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
)

type createCollectionRequest struct {
	Name     string                 `json:"name"`
	Defaults map[string]any         `json:"defaults"`
	Storage  collection.StoreSpec   `json:"storage"`
	Records  collection.RecordsSpec `json:"records"`
}

func newCollectionDefaults() map[string]any {
	return map[string]any{
		"id": "uuid()",
	}
}

func createCollection(ctx context.Context, w http.ResponseWriter, input *createCollectionRequest) (*CollectionResponse, error) {

	s := GetServicer(ctx)

	spec := collection.CollectionSpec{
		Name:    input.Name,
		Store:   input.Storage,
		Records: input.Records,
	}
	col, err := s.CreateCollectionSpec(input.Name, spec)
	if err == service.ErrorCollectionAlreadyExists {
		w.WriteHeader(http.StatusConflict)
		return nil, err // todo: return custom error, with detailed description
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return nil, err // todo: wrap error?
	}

	if input.Defaults == nil {
		input.Defaults = newCollectionDefaults()
	}
	col.SetDefaults(input.Defaults)

	w.WriteHeader(http.StatusCreated)
	return &CollectionResponse{
		Name:     input.Name,
		Total:    int(col.Count()),
		Defaults: col.Defaults(),
	}, nil
}
