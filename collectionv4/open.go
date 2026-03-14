package collectionv4

import "path/filepath"

func OpenCollection(filename string) (*Collection, error) {
	store, err := NewStoreDisk(filename)
	if err != nil {
		return nil, err
	}

	col := NewCollection(filepath.Base(filename), store)
	col.SetFilepath(filename)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}
