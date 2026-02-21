package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

func getCollection(ctx context.Context) (*CollectionResponse, error) {

	s := GetServicer(ctx)

	collectionName := box.GetUrlParameter(ctx, "collectionName")

	collection, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
		// todo: wrap error
		return nil, err
	}

	return &CollectionResponse{
		Name:     collectionName,
		Total:    collection.Rows.Len(),
		Indexes:  len(collection.Indexes),
		Defaults: collection.Defaults,
	}, nil
}
