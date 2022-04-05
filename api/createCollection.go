package api

import (
	"path"

	"inceptiondb/collection"
)

type createCollectionRequest struct {
	Name string `json:"name"`
}

func createCollection(collections map[string]*collection.Collection, dir string) interface{} {
	return func(input *createCollectionRequest) (*createCollectionRequest, error) {

		filename := path.Join(dir, input.Name)

		col, err := collection.OpenCollection(filename)
		if err != nil {
			return nil, err
		}

		collections[input.Name] = col

		return input, nil
	}
}
