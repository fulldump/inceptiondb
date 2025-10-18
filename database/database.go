package database

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
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

var (
	ErrCollectionAlreadyExists = errors.New("collection already exists")
	ErrCollectionNotFound      = errors.New("collection not found")
)

type Config struct {
	Dir string
}

type Database struct {
	Config        *Config
	status        string
	Collections   map[string]*collection.Collection
	exit          chan struct{}
	collectionsMu sync.RWMutex
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

	db.collectionsMu.RLock()
	_, exists := db.Collections[name]
	db.collectionsMu.RUnlock()
	if exists {
		return nil, ErrCollectionAlreadyExists
	}

	filename := path.Join(db.Config.Dir, name)
	col, err := collection.OpenCollection(filename)
	if err != nil {
		return nil, err
	}

	db.collectionsMu.Lock()
	defer db.collectionsMu.Unlock()
	if _, exists := db.Collections[name]; exists {
		col.Close()
		return nil, ErrCollectionAlreadyExists
	}

	db.Collections[name] = col

	return col, nil
}

func (db *Database) DropCollection(name string) error { // TODO: rename drop?

	db.collectionsMu.Lock()
	col, exists := db.Collections[name]
	if !exists {
		db.collectionsMu.Unlock()
		return ErrCollectionNotFound
	}
	delete(db.Collections, name)
	db.collectionsMu.Unlock()

	filename := path.Join(db.Config.Dir, name)

	if err := os.Remove(filename); err != nil {
		return err // TODO: wrap?
	}

	return col.Close()
}

func (db *Database) GetCollection(name string) (*collection.Collection, error) {
	db.collectionsMu.RLock()
	defer db.collectionsMu.RUnlock()

	col, exists := db.Collections[name]
	if !exists {
		return nil, ErrCollectionNotFound
	}

	return col, nil
}

func (db *Database) ListCollections() map[string]*collection.Collection {
	db.collectionsMu.RLock()
	defer db.collectionsMu.RUnlock()

	result := make(map[string]*collection.Collection, len(db.Collections))
	for name, col := range db.Collections {
		result[name] = col
	}

	return result
}

func (db *Database) Load() error {

	fmt.Printf("Loading database %s...\n", db.Config.Dir) // todo: move to logger
	dir := db.Config.Dir
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	type job struct {
		filename string
		name     string
	}
	jobs := make([]job, 0)
	err := filepath.WalkDir(dir, func(filename string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		name := strings.TrimPrefix(filename, dir)
		name = strings.TrimPrefix(name, "/")
		jobs = append(jobs, job{filename: filename, name: name})
		return nil
	})
	if err != nil {
		db.status = StatusClosing
		return err
	}
	if len(jobs) == 0 {
		fmt.Println("Ready")
		db.status = StatusOperating
		return nil
	}
	workers := runtime.NumCPU()
	if workers > len(jobs) {
		workers = len(jobs)
	}
	if workers == 0 {
		workers = 1
	}
	jobCh := make(chan job)
	var wg sync.WaitGroup
	var once sync.Once
	var loadErr error
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobCh {
				start := time.Now()
				col, err := collection.OpenCollection(job.filename)
				if err != nil {
					fmt.Printf("ERROR: open collection '%s': %s\n", job.filename, err.Error()) // todo: move to logger
					once.Do(func() { loadErr = err })
					continue
				}
				fmt.Println(job.name, len(col.Rows), time.Since(start)) // todo: move to logger
				db.collectionsMu.Lock()
				db.Collections[job.name] = col
				db.collectionsMu.Unlock()
			}
		}()
	}
	for _, job := range jobs {
		jobCh <- job
	}
	close(jobCh)
	wg.Wait()
	if loadErr != nil {
		db.status = StatusClosing
		return loadErr
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
