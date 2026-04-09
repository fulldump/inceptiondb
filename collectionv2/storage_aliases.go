package collectionv2

import (
	"github.com/fulldump/inceptiondb/collectionv2/storage"
)

type Storage = storage.Storage
type LoadedCommand = storage.LoadedCommand
type Command = storage.Command
type CreateIndexCommand = storage.CreateIndexCommand
type DropIndexCommand = storage.DropIndexCommand

func NewJSONStorage(filename string) (Storage, error) {
	return storage.NewJSONStorage(filename)
}

func NewGobStorage(filename string) (Storage, error) {
	return storage.NewGobStorage(filename)
}

func NewGzipStorage(filename string) (Storage, error) {
	return storage.NewGzipStorage(filename)
}

func NewSnapshotStorage(filename string) (Storage, error) {
	return storage.NewSnapshotStorage(filename)
}

func NewWALStorage(filename string) (Storage, error) {
	return storage.NewWALStorage(filename)
}
