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
			Total:    len(collection.Rows),
			Indexes:  len(collection.Indexes),
			Defaults: collection.Defaults,
		})
	}
	return response, nil
}
