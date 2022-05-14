package api

import (
	"fmt"
	"net/http"
	"path"

	"inceptiondb/collection"
)

type createCollectionRequest struct {
	Name string `json:"name"`
}

func createCollection(collections map[string]*collection.Collection, dir string) interface{} {
	return func(w http.ResponseWriter, input *createCollectionRequest) (*createCollectionRequest, error) {

		_, exist := collections[input.Name]
		if exist {
			w.WriteHeader(http.StatusConflict)
			return nil, fmt.Errorf("collection '%s' already exists", input.Name)
		}

		filename := path.Join(dir, input.Name)

		col, err := collection.OpenCollection(filename)
		if err != nil {
			return nil, err
		}

		collections[input.Name] = col

		return input, nil
	}
}
