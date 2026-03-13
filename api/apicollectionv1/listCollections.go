package apicollectionv1

import (
	"context"
	"net/http"
)

func listCollections(ctx context.Context, w http.ResponseWriter) ([]*CollectionResponse, error) {

	s := GetServicer(ctx)

	response := []*CollectionResponse{}
	for name, collection := range s.ListCollections() {
		response = append(response, &CollectionResponse{
			Name:     name,
			Total:    0, // collection.Rows.Len(), // todo: fix this
			Indexes:  len(collection.Indexes),
			Defaults: collection.Defaults,
		})
	}
	return response, nil
}
