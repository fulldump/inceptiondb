package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/inceptiondb/service"
)

func listCollections(s service.Servicer) interface{} {
	return func(ctx context.Context, w http.ResponseWriter) (interface{}, error) {

		result, err := s.ListCollections()
		if err != nil {
			return nil, err // todo: wrap this?
		}

		return result, nil
	}
}
