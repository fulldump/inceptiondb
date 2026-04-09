package storage

import "testing"

func TestGzipStorageSuite(t *testing.T) {
	runStorageSuite(t, func(filename string) (Storage, error) {
		return NewGzipStorage(filename)
	})
}
