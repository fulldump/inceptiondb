package api

import (
	"github.com/fulldump/inceptiondb/collection"
)

func listCollections(collections map[string]*collection.Collection) interface{} {
	return func() []string {
		result := []string{}

		for k, _ := range collections {
			result = append(result, k)
		}

		return result
	}
}
