package collectionv4

import (
	"path/filepath"
)

func OpenCollection(filename string) (*Collection, error) {
	rawStore, err := NewStoreDisk(filename)
	//rawStore, err := NewStoreJson(filename)
	//rawStore, err := NewStoreCrazy(filename)
	if err != nil {
		return nil, err
	}

	var store Store = rawStore

	//store = NewStoreSnappy(store)
	store = NewStoreAsync(store)
	//store = NewStoreFlusher(store, 10*time.Second)

	col := NewCollection(filepath.Base(filename), store)
	col.SetFilepath(filename)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}
