package storage

import "testing"

func TestJSONStorageSuite(t *testing.T) {
	runStorageSuite(t, func(filename string) (Storage, error) {
		return NewJSONStorage(filename)
	})
}
