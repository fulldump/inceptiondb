package database

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
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
	Config      *Config
	status      string
	mu          sync.RWMutex
	Collections map[string]*collection.Collection
	exit        chan struct{}
}

func NewDatabase(config *Config) *Database { // todo: return error?
	s := &Database{
		Config:      config,
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
	return db.CreateCollectionSpec(name, collection.DefaultCollectionSpec(path.Join(db.Config.Dir, name)))
}

func (db *Database) CreateCollectionSpec(name string, spec collection.CollectionSpec) (*collection.Collection, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, exists := db.Collections[name]
	if exists {
		return nil, fmt.Errorf("collection '%s' already exists", name)
	}

	filename := path.Join(db.Config.Dir, name)
	spec.Name = name
	spec.Filename = filename
	if spec.Store.Backend == "" {
		spec.Store = collection.DefaultCollectionSpec(filename).Store
	}
	if spec.Records.Engine == "" {
		spec.Records = collection.DefaultCollectionSpec(filename).Records
	}

	if err := collection.SaveCollectionSpec(spec); err != nil {
		return nil, err
	}

	col, err := collection.OpenCollectionSpec(spec)
	if err != nil {
		return nil, err
	}

	db.Collections[name] = col

	return col, nil
}

func (db *Database) DropCollection(name string) error { // TODO: rename drop?
	db.mu.Lock()
	defer db.mu.Unlock()

	col, exists := db.Collections[name]
	if !exists {
		return fmt.Errorf("collection '%s' not found", name)
	}

	filename := path.Join(db.Config.Dir, name)

	err := os.Remove(filename)
	if err != nil {
		return err // TODO: wrap?
	}
	if err := os.Remove(collection.CollectionSpecPath(filename)); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(db.Collections, name) // TODO: protect section! not threadsafe

	return col.Close()
}

func (db *Database) GetCollection(name string) (*collection.Collection, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	col, ok := db.Collections[name]
	return col, ok
}

func (db *Database) ListCollections() map[string]*collection.Collection {
	db.mu.RLock()
	defer db.mu.RUnlock()
	out := make(map[string]*collection.Collection, len(db.Collections))
	for name, col := range db.Collections {
		out[name] = col
	}
	return out
}

func (db *Database) Load() error {

	fmt.Printf("Loading database %s...\n", db.Config.Dir) // todo: move to logger
	dir := db.Config.Dir
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
		if collection.IsCollectionSpecFile(filename) {
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
		fmt.Println(name, "collection open took", time.Since(t0)) // todo: move to logger

		db.mu.Lock()
		db.Collections[name] = col
		db.mu.Unlock()

		return nil
	})

	if err != nil {
		db.status = StatusClosing
		return err
	}

	fmt.Println("Ready")

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
	for name, col := range db.ListCollections() {
		fmt.Printf("Closing '%s'...\n", name)
		err := col.Close()
		if err != nil {
			fmt.Printf("ERROR: close(%s): %s", name, err.Error())
			lastErr = err
		}
	}

	return lastErr
}
