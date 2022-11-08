package apicollectionv1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
	"github.com/fulldump/inceptiondb/utils"
)

type CreateIndexRequest struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

func createIndex(ctx context.Context, input *CreateIndexRequest) (*listIndexesItem, error) {

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		col, err = s.CreateCollection(collectionName)
	}
	if err != nil {
		return nil, err // todo: handle/wrap this properly
	}

	var options interface{}

	switch input.Type {
	case "map":
		options = &collection.IndexMapOptions{}
	case "btree":
		options = &collection.IndexBTreeOptions{}
	default:
		return nil, fmt.Errorf("unexpected type '%s' instead of [map|btree]", input.Type)
	}

	utils.Remarshal(input.Options, options) // todo: handle error properly

	err = col.Index(input.Name, options)
	if err != nil {
		return nil, err
	}

	box.GetResponse(ctx).WriteHeader(http.StatusCreated)

	return &listIndexesItem{
		Name:    input.Name,
		Type:    input.Type,
		Options: options,
	}, nil
}
