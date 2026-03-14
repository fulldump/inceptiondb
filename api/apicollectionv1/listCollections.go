package apicollectionv1

import (
	"context"
	"net/http"
)

func listCollections(ctx context.Context, w http.ResponseWriter) ([]*CollectionResponse, error) {

	s := GetServicer(ctx)

	response := []*CollectionResponse{}
	for name, collection := range s.ListCollections() {
		indexes := collection.ListIndexes()
		response = append(response, &CollectionResponse{
			Name:     name,
			Total:    int(collection.Count()),
			Indexes:  len(indexes),
			Defaults: collection.Defaults(),
		})
	}
	return response, nil
}
