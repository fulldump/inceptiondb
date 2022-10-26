package service

import (
	"errors"
)

type Collection struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
}

var ErrorCollectionNotFound = errors.New("collection not found")

type Servicer interface { // todo: review naming
	CreateCollection(name string) (*Collection, error)
	GetCollection(name string) (*Collection, error)
	ListCollections() ([]*Collection, error)
}
