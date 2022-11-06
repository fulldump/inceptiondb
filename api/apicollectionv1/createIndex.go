package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
)

func createIndex(ctx context.Context, input *collection.CreateIndexOptions) (*listIndexesItem, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		col, err = s.CreateCollection(collectionName)
	}
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	err = col.Index(input)
	if err != nil {
		return nil, err
	}

	box.GetResponse(ctx).WriteHeader(http.StatusCreated)

	return &listIndexesItem{
		Name: input.Name,
		Kind: input.Kind,
		// todo: return parameteres somehow
	}, nil
}
