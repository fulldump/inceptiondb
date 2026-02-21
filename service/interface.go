package service

import (
	"errors"

	"github.com/fulldump/inceptiondb/collectionv2"
)

var ErrorCollectionNotFound = errors.New("collection not found")

type Servicer interface { // todo: review naming
	CreateCollection(name string) (*collectionv2.Collection, error)
	GetCollection(name string) (*collectionv2.Collection, error)
	ListCollections() map[string]*collectionv2.Collection
	DeleteCollection(name string) error
}
