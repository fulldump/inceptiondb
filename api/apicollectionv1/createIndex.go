package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
)

func createIndex(ctx context.Context, options *collection.IndexOptions) (*listIndexesItem, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	err = collection.Index(options)
	if err != nil {
		return nil, err
	}

	box.GetResponse(ctx).WriteHeader(http.StatusCreated)

	return &listIndexesItem{
		Name:   options.Field,
		Field:  options.Field,
		Sparse: options.Sparse,
	}, nil
}
