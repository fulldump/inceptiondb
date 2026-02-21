package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/fulldump/inceptiondb/collectionv2"
	"github.com/fulldump/inceptiondb/database"
)

type Service struct {
	db          *database.Database
	collections map[string]*collectionv2.Collection
}

func NewService(db *database.Database) *Service {
	return &Service{
		db:          db,
		collections: db.Collections, // todo: remove from here
	}
}

var ErrorCollectionAlreadyExists = errors.New("collection already exists")

func (s *Service) CreateCollection(name string) (*collectionv2.Collection, error) {
	_, exist := s.collections[name]
	if exist {
		return nil, ErrorCollectionAlreadyExists
	}

	filename := path.Join(s.db.Config.Dir, name)

	collection, err := collectionv2.OpenCollection(filename)
	if err != nil {
		return nil, err
	}

	s.collections[name] = collection

	return collection, nil
}

func (s *Service) GetCollection(name string) (*collectionv2.Collection, error) {
	collection, exist := s.collections[name]
	if !exist {
		return nil, ErrorCollectionNotFound
	}

	return collection, nil
}

func (s *Service) ListCollections() map[string]*collectionv2.Collection {
	return s.collections
}

func (s *Service) DeleteCollection(name string) error {
	return s.db.DropCollection(name)
}

var ErrorInsertBadJson = errors.New("insert bad json")
var ErrorInsertConflict = errors.New("insert conflict")

func (s *Service) Insert(name string, data io.Reader) error {

	collection, exists := s.db.Collections[name]
	if !exists {
		// TODO: here create collection :D
		return ErrorCollectionNotFound
	}

	jsonReader := json.NewDecoder(data)

	for {
		item := map[string]interface{}{}
		err := jsonReader.Decode(&item)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			return ErrorInsertBadJson
		}
		_, err = collection.Insert(item)
		if err != nil {
			// TODO: handle error properly
			return ErrorInsertConflict
		}

		// jsonWriter.Encode(item)
	}

	return nil
}
