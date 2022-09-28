package database

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/fulldump/inceptiondb/collection"
)

const (
	StatusOpening   = "opening"
	StatusOperating = "operating"
	StatusClosing   = "closing"
)

type Config struct {
	Dir string
}

type Database struct {
	config      *Config
	status      string
	Collections map[string]*collection.Collection
	exit        chan struct{}
}

func NewDatabase(config *Config) *Database { // todo: return error?
	s := &Database{
		config:      config,
		status:      StatusOpening,
		Collections: map[string]*collection.Collection{},
		exit:        make(chan struct{}),
	}

	return s
}

func (db *Database) GetStatus() string {
	return db.status
}

func (db *Database) CreateCollection(name string) (*collection.Collection, error) {

	col, exists := db.Collections[name]
	if exists {
		return nil, fmt.Errorf("collection '%s' already exists", name)
	}

	filename := path.Join(db.config.Dir, name)
	col, err := collection.OpenCollection(filename)
	if err != nil {
		return nil, err
	}

	db.Collections[name] = col

	return col, nil
}

func (db *Database) DropCollection(name string) (error) { // TODO: rename drop?

	col, exists := db.Collections[name]
	if !exists {
		return fmt.Errorf("collection '%s' not found", name)
	}

	filename := path.Join(db.config.Dir, name)

	err := os.Remove(filename)
	if err != nil {
		return err // TODO: wrap?
	}

	delete(db.Collections, name) // TODO: protect section! not threadsafe

	return  col.Close()
}

func (db *Database) Load() error {

	fmt.Printf("Loading database %s...\n", db.config.Dir) // todo: move to logger
	dir := db.config.Dir
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}
	err = filepath.WalkDir(dir, func(filename string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		name := filename
		name = strings.TrimPrefix(name, dir)
		name = strings.TrimPrefix(name, "/")

		t0 := time.Now()
		col, err := collection.OpenCollection(filename)
		if err != nil {
			fmt.Printf("ERROR: open collection '%s': %s\n", filename, err.Error()) // todo: move to logger
			return err
		}
		fmt.Println(name, len(col.Rows), time.Since(t0)) // todo: move to logger

		db.Collections[name] = col

		return nil
	})

	if err != nil {
		db.status = StatusClosing
		return err
	}

	db.status = StatusOperating

	return nil

}

func (db *Database) Start() error {

	go db.Load()

	<-db.exit

	return nil
}

func (db *Database) Stop() error {

	defer close(db.exit)

	db.status = StatusClosing

	var lastErr error
	for name, col := range db.Collections {
		fmt.Printf("Closing '%s'...\n", name)
		err := col.Close()
		if err != nil {
			fmt.Printf("ERROR: close(%s): %s", name, err.Error())
			lastErr = err
		}
	}

	return lastErr
}
