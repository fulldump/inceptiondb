package api

import (
	"github.com/fulldump/box"

	"inceptiondb/collection"
)

func Build(collections map[string]*collection.Collection, dataDir string) *box.B {

	b := box.NewBox()

	b.WithInterceptors(interceptorPrintError)

	b.Resource("collections").
		WithActions(
			box.Get(listCollections(collections)),
			box.Post(createCollection(collections, dataDir)),
		)

	b.Resource("collections/{collection_name}").
		WithActions(
			box.Get(listItems(collections)),
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
		)

	return b
}
