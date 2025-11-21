package apicollectionv1

import (
	"context"
	"encoding/json"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/utils"
)

type listIndexesItem struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

func (l *listIndexesItem) MarshalJSON() ([]byte, error) {

	result := map[string]interface{}{
		"name": l.Name,
		"type": l.Type,
	}
	utils.Remarshal(l.Options, &result)

	return json.Marshal(result)
}

func listIndexes(ctx context.Context) ([]*listIndexesItem, error) {

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
			// Type:    index.Type,
			// Options: index.Options,
		})
	}

	return result, nil
}
