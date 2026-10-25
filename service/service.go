package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"

	"github.com/fulldump/inceptiondb/collection"
	"github.com/fulldump/inceptiondb/database"
)

type Service struct {
	db *database.Database
}

func NewService(db *database.Database) *Service {
	return &Service{
		db: db,
	}
}

var ErrorCollectionAlreadyExists = errors.New("collection already exists")

func (s *Service) CreateCollection(name string) (*collection.Collection, error) {
	return s.CreateCollectionSpec(name, collection.DefaultCollectionSpec(path.Join(s.db.Config.Dir, name)))
}

func (s *Service) CreateCollectionSpec(name string, spec collection.CollectionSpec) (*collection.Collection, error) {
	_, exist := s.db.GetCollection(name)
	if exist {
		return nil, ErrorCollectionAlreadyExists
	}

	collection, err := s.db.CreateCollectionSpec(name, spec)
	if err != nil {
		return nil, err
	}

	return collection, nil
}

func (s *Service) GetCollection(name string) (*collection.Collection, error) {
	collection, exist := s.db.GetCollection(name)
	if !exist {
		return nil, ErrorCollectionNotFound
	}

	return collection, nil
}

func (s *Service) ListCollections() map[string]*collection.Collection {
	return s.db.ListCollections()
}

func (s *Service) DeleteCollection(name string) error {
	return s.db.DropCollection(name)
}

var ErrorInsertBadJson = errors.New("insert bad json")
var ErrorInsertConflict = errors.New("insert conflict")

func (s *Service) Insert(name string, data io.Reader) error {

	collection, exists := s.db.GetCollection(name)
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
		_, err = collection.InsertMap(item, false)
		if err != nil {
			// TODO: handle error properly
			return ErrorInsertConflict
		}

		// jsonWriter.Encode(item)
	}
}
