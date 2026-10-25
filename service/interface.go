package service

import (
	"errors"

	"github.com/fulldump/inceptiondb/collection"
)

var ErrorCollectionNotFound = errors.New("collection not found")

type Servicer interface { // todo: review naming
	CreateCollection(name string) (*collection.Collection, error)
	CreateCollectionSpec(name string, spec collection.CollectionSpec) (*collection.Collection, error)
	GetCollection(name string) (*collection.Collection, error)
	ListCollections() map[string]*collection.Collection
	DeleteCollection(name string) error
}
