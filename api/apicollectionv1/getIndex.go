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
	current, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	name := input.Name
	index, found := current.Indexes[name]

	if !found {
		box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
		return nil, fmt.Errorf("index '%s' not found in collection '%s'", input.Name, collectionName)
	}

	return &listIndexesItem{
		Name:    name,
		Type:    index.Type,
		Options: index.Options,
	}, nil
}
