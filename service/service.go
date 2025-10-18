package service

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"

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
	col, err := s.db.CreateCollection(name)
	if err != nil {
		if errors.Is(err, database.ErrCollectionAlreadyExists) {
			return nil, ErrorCollectionAlreadyExists
		}
		return nil, err
	}

	return col, nil
}

func (s *Service) GetCollection(name string) (*collection.Collection, error) {
	col, err := s.db.GetCollection(name)
	if err != nil {
		if errors.Is(err, database.ErrCollectionNotFound) {
			return nil, ErrorCollectionNotFound
		}
		return nil, err
	}

	return col, nil
}

func (s *Service) ListCollections() map[string]*collection.Collection {
	return s.db.ListCollections()
}

func (s *Service) DeleteCollection(name string) error {
	err := s.db.DropCollection(name)
	if errors.Is(err, database.ErrCollectionNotFound) {
		return ErrorCollectionNotFound
	}
	return err
}

var ErrorInsertBadJson = errors.New("insert bad json")
var ErrorInsertConflict = errors.New("insert conflict")

func (s *Service) Insert(name string, data io.Reader) error {

	collection, err := s.db.GetCollection(name)
	if err != nil {
		// TODO: here create collection :D
		if errors.Is(err, database.ErrCollectionNotFound) {
			return ErrorCollectionNotFound
		}
		return err
	}

	jsonReader := jsontext.NewDecoder(data,
		jsontext.AllowDuplicateNames(true),
		jsontext.AllowInvalidUTF8(true),
	)

	for {
		item := map[string]any{}
		err := jsonv2.UnmarshalDecode(jsonReader, &item)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// TODO: handle error properly
			fmt.Println("ERROR:", err.Error())
			return ErrorInsertBadJson
		}
		if _, err = collection.Insert(item); err != nil {
			// TODO: handle error properly
			return ErrorInsertConflict
		}

		// jsonWriter.Encode(item)
	}

	return nil
}
