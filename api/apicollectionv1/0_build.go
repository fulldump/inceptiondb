package apicollectionv1

import (
	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/service"
)

// todo: rename to BuildV1Collection
func BuildV1Collection(v1 *box.R, s service.Servicer) *box.R {

	collections := v1.Resource("/collections").
		WithActions(
			box.Get(listCollections),
			box.Post(createCollection),
		)

	v1.Resource("/collections/{collectionName}").
		WithActions(
			box.Get(getCollection),
			box.ActionPost(insert),
			box.ActionPost(find),
			box.ActionPost(remove),
			box.ActionPost(patch),
			box.ActionPost(dropCollection),
			box.ActionPost(listIndexes),
			box.ActionPost(createIndex),
			box.ActionPost(getIndex),
			box.ActionPost(size),
		)

	return collections
}
