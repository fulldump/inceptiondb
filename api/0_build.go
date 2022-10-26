package api

import (
	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/api/apicollectionv1"
	"github.com/fulldump/inceptiondb/database"
	"github.com/fulldump/inceptiondb/service"
	"github.com/fulldump/inceptiondb/statics"
)

func Build(db *database.Database, dataDir string, staticsDir string) *box.B { // TODO: remove datadir

	collections := db.Collections

	b := box.NewBox()

	b.WithInterceptors(
		recoverFromPanic,
		interceptorPrintError,
	)

	b.Resource("collections").
		WithActions(
			box.Get(listCollections(collections)),
			box.Post(createCollection(collections, dataDir)),
		)

	b.Resource("collections/{collection_name}").
		WithActions(
			box.Get(listItems(collections)),
			box.Delete(deleteCollection(collections)),
			box.Post(insertItem(collections)),
		)

	b.Resource("collections/{collection_name}/count").
		WithActions(
			box.Get(countItems(collections)),
		)

	b.Resource("collections/{collection_name}/index").
		WithActions(
			box.Get(listIndexes(collections)),
			box.Post(createIndex(collections)),
		)

	b.Resource("collections/{collection_name}/index/{index_name}").
		WithActions(
			box.Get(getIndex(collections)),
		)

	b.Resource("collections/{collection_name}/index/{index_name}/findBy/{value}").
		WithActions(
			box.Get(indexFindBy(collections)),
			box.Delete(indexDeleteBy(collections)),
			box.Patch(indexPatchBy(collections)),
		)

	v1 := b.Resource("/v1")
	s := service.NewService(db)
	apicollectionv1.BuildV1Collection(v1, s)

	// Mount statics
	b.Resource("/*").
		WithActions(
			box.Get(statics.ServeStatics(staticsDir)),
		)

	return b
}
