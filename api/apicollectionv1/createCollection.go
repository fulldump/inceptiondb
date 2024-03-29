package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/inceptiondb/service"
)

type createCollectionRequest struct {
	Name     string         `json:"name"`
	Defaults map[string]any `json:"defaults"`
}

func newCollectionDefaults() map[string]any {
	return map[string]any{
		"id": "uuid()",
	}
}

func createCollection(ctx context.Context, w http.ResponseWriter, input *createCollectionRequest) (*CollectionResponse, error) {

	s := GetServicer(ctx)

	collection, err := s.CreateCollection(input.Name)
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
	collection.SetDefaults(input.Defaults)

	w.WriteHeader(http.StatusCreated)
	return &CollectionResponse{
		Name:     input.Name,
		Total:    len(collection.Rows),
		Defaults: collection.Defaults,
	}, nil
}
