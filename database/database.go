package database

import (
	"fmt"
	"inceptiondb/collection"
	"io/fs"
	"path/filepath"
	"strings"
	"time"
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

func (db *Database) load() error {

	fmt.Printf("Loading data...\n") // todo: move to logger
	dir := db.config.Dir
	err := filepath.WalkDir(dir, func(filename string, d fs.DirEntry, err error) error {
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

	go db.load()

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
