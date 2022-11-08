package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"
)

type listIndexesItem struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
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
		_ = index
		result = append(result, &listIndexesItem{
			Name: name,
			// TODO: complete the rest of fields
		})
	}

	return result, nil
}
