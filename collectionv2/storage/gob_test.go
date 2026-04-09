package storage

import "testing"

func TestGobStorageSuite(t *testing.T) {
	runStorageSuite(t, func(filename string) (Storage, error) {
		return NewGobStorage(filename)
	})
}
