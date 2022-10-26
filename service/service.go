package service

import (
	"errors"
	"path"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/database"
)

type Service struct {
	db          *database.Database
	collections map[string]*collection.Collection
}

func NewService(db *database.Database) *Service {
	return &Service{
		db:          db,
		collections: db.Collections, // todo: remove from here
	}
}

var ErrorCollectionAlreadyExists = errors.New("collection already exists")

func (s *Service) CreateCollection(name string) (*Collection, error) {

	_, exist := s.collections[name]
	if exist {
		return nil, ErrorCollectionAlreadyExists
	}

	filename := path.Join(s.db.Config.Dir, name)

	collection, err := collection.OpenCollection(filename)
	if err != nil {
		return nil, err
	}

	s.collections[name] = collection

	return &Collection{
		Name: name,
	}, nil
}

func (s *Service) GetCollection(name string) (*Collection, error) {
	collection, exist := s.collections[name]
	if !exist {
		return nil, ErrorCollectionNotFound
	}

	return &Collection{
		Name:  name,
		Total: len(collection.Rows),
	}, nil
}

func (s *Service) ListCollections() ([]*Collection, error) {
	result := []*Collection{}

	for k, collection := range s.collections {
		result = append(result, &Collection{
			Name:  k,
			Total: len(collection.Rows),
		})
	}

	return result, nil
}
