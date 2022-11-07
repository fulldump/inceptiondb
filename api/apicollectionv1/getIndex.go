package apicollectionv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/fulldump/box"
	"github.com/fulldump/inceptiondb/collection"
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

	for name, index := range current.Indexes {
		_ = index

		if name == input.Name {
			if btreeIndex, ok := index.(*collection.IndexBtree); ok {

				rawOptions, err := json.Marshal(btreeIndex.Options)
				if err != nil {
					return nil, err // todo handler this error
				}

				return &listIndexesItem{
					Name:       name,
					Kind:       "btree", // todo get this one from a constant
					Parameters: rawOptions,
				}, nil
			}

			if mapIndex, ok := index.(*collection.IndexMap); ok {

				rawOptions, err := json.Marshal(mapIndex.Options)
				if err != nil {
					return nil, err // todo handler this error
				}

				return &listIndexesItem{
					Name:       name,
					Kind:       "map", // todo get this one from a constant
					Parameters: rawOptions,
				}, nil

			}

			return &listIndexesItem{
				Name: name,
				// Field:  name,
				// Sparse: index.Sparse,
				// todo: fild properly
			}, nil
		}
	}

	box.GetResponse(ctx).WriteHeader(http.StatusNotFound)

	return nil, fmt.Errorf("index '%s' not found in collection '%s'", input.Name, collectionName)
}
