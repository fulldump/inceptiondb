package apicollectionv1

import (
	"context"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

func BuildV1Collection(v1 *box.R, s service.Servicer) *box.R {

	collections := v1.Resource("/collections").
		WithActions(
			box.Get(listCollections(s)).WithName("listCollections"),
			box.Post(createCollection(s)).WithName("createCollection"),
		)

	v1.Resource("/collections/{collectionName}").
		WithActions(
			box.Get(func(ctx context.Context) (interface{}, error) {

				collectionName := box.GetUrlParameter(ctx, "collectionName")

				collection, err := s.GetCollection(collectionName)
				if err == service.ErrorCollectionNotFound {
					box.GetResponse(ctx).WriteHeader(http.StatusNotFound)
					// todo: wrap error
					return nil, err
				}

				return collection, nil
			}),
			// box.ActionPost(insert),
			// box.ActionPost(find),
			// box.ActionPost(remove),
			// box.ActionPost(patch), // rename to update?
			// box.ActionPost(dropCollection),
			// box.ActionPost(listIndexes),
			// box.ActionPost(createIndex),
			// box.ActionPost(getIndex),
		)

	return collections
}
