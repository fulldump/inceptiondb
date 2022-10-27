package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"
)

type listIndexesItem struct {
	Name   string `json:"name"`
	Field  string `json:"field"`
	Sparse bool   `json:"sparse"`
}

func listIndexes(ctx context.Context, w http.ResponseWriter) ([]*listIndexesItem, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	collection, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	result := []*listIndexesItem{}
	for name, index := range collection.Indexes {
		result = append(result, &listIndexesItem{
			Name:   name,
			Field:  name,
			Sparse: index.Sparse,
		})
	}

	return result, nil
}
