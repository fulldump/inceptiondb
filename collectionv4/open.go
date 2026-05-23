package collectionv4

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fulldump/inceptiondb/collectionv4/records"
	"github.com/fulldump/inceptiondb/collectionv4/stores"
)

func OpenCollectionCustom(filename, rawstore_name, wrapstore_name, records_name string) (*Collection, error) {
	var store stores.Store
	var err error

	switch rawstore_name {
	case "disk":
		store, err = stores.NewStoreDisk(filename)
	case "json":
		store, err = stores.NewStoreJson(filename)
	case "crazy":
		store, err = stores.NewStoreCrazy(filename)
	default:
		return nil, fmt.Errorf("unknown raw store type: %s", rawstore_name)
	}
	if err != nil {
		return nil, err
	}

	switch wrapstore_name {
	case "snappy":
		store = stores.NewStoreSnappy(store)
	case "async":
		store = stores.NewStoreAsync(store)
	case "flusher":
		store = stores.NewStoreFlusher(store, 10*time.Second)
	case "":
	// do nothing
	default:
		return nil, fmt.Errorf("unknown wrap store type: %s", rawstore_name)
	}

	var rr records.Records[Record]
	switch records_name {
	case "correct":
		rr = records.NewRecordsCorrect[Record]()
	case "fast":
		rr = records.NewRecordsFast[Record]()
	case "Hyper":
		rr = records.NewRecordsHyper[Record]()
	case "Turbo":
		rr = records.NewRecordsTurbo[Record]()
	case "ultra", "":
		rr = records.NewRecordsUltra[Record]()
	default:
		return nil, fmt.Errorf("unknown records type: %s", records_name)
	}

	col := NewCollectionBase(filepath.Base(filename), store, rr)
	col.SetFilepath(filename)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}

func OpenCollection(filename string) (*Collection, error) {
	return OpenCollectionCustom(filename, "disk", "async", "ultra")
}
