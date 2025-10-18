package apicollectionv1

import (
	"context"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/service"
)

type CreateIndexRequest struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

func createIndex(ctx context.Context, r *http.Request) (*listIndexesItem, error) {

	requestBody, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	input := struct {
		Name string
		Type string
	}{
		"",
		"", // todo: put default index here (if any)
	}
	err = jsonv2.Unmarshal(requestBody, &input)
	if err != nil {
		return nil, err
	}

	s := GetServicer(ctx)
	collectionName := box.GetUrlParameter(ctx, "collectionName")
	col, err := s.GetCollection(collectionName)
	if err == service.ErrorCollectionNotFound {
		col, err = s.CreateCollection(collectionName)
		if err != nil {
			return nil, err // todo: handle/wrap this properly
		}
		err = col.SetDefaults(newCollectionDefaults())
		if err != nil {
			return nil, err // todo: handle/wrap this properly
		}
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

	err = jsonv2.Unmarshal(requestBody, &options)
	if err != nil {
		return nil, err
	}

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
