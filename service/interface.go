package service

import (
	"errors"

	"github.com/fulldump/inceptiondb/collectionv4"
)

var ErrorCollectionNotFound = errors.New("collection not found")

type Servicer interface { // todo: review naming
	CreateCollection(name string) (*collectionv4.Collection, error)
	GetCollection(name string) (*collectionv4.Collection, error)
	ListCollections() map[string]*collectionv4.Collection
	DeleteCollection(name string) error
}
