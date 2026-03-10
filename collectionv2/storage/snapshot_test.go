package storage

import "testing"

func TestSnapshotStorageSuite(t *testing.T) {
	runStorageSuite(t, func(filename string) (Storage, error) {
		return NewSnapshotStorage(filename)
	})
}
