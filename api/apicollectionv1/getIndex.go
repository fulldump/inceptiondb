package apicollectionv1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fulldump/box"
)

type getIndexInput struct {
	Name string
}

func getIndex(ctx context.Context, input getIndexInput) (*listIndexesItem, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	for name, index := range collection.Indexes {
		if name == input.Name {
			return &listIndexesItem{
				Name:   name,
				Field:  name,
				Sparse: index.Sparse,
			}, nil
		}
	}

	box.GetResponse(ctx).WriteHeader(http.StatusNotFound)

	return nil, fmt.Errorf("index '%s' not found in collection '%s'", input.Name, collectionName)
}
