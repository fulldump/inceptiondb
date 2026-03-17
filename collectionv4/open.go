package collectionv4

import (
	"path/filepath"

	"github.com/fulldump/inceptiondb/collectionv4/stores"
)

func OpenCollection(filename string) (*Collection, error) {
	rawStore, err := stores.NewStoreDisk(filename)
	//rawStore, err := stores.NewStoreJson(filename)
	//rawStore, err := stores.NewStoreCrazy(filename)
	if err != nil {
		return nil, err
	}

	var store stores.Store = rawStore

	//store = stores.NewStoreSnappy(store)
	store = stores.NewStoreAsync(store)
	//store = stores.NewStoreFlusher(store, 10*time.Second)

	col := NewCollection(filepath.Base(filename), store)
	col.SetFilepath(filename)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}
