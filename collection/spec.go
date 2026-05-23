package collection

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/fulldump/inceptiondb/collection/records"
	"github.com/fulldump/inceptiondb/collection/stores"
)

const (
	StoreBackendDisk  = "disk"
	StoreBackendJSON  = "json"
	StoreBackendCrazy = "crazy"

	StoreWrapperSnappy  = "snappy"
	StoreWrapperAsync   = "async"
	StoreWrapperFlusher = "flusher"

	RecordsCorrect = "correct"
	RecordsFast    = "fast"
	RecordsHyper   = "hyper"
	RecordsTurbo   = "turbo"
	RecordsUltra   = "ultra"
)

type CollectionSpec struct {
	Name     string
	Filename string
	Store    StoreSpec
	Records  RecordsSpec
}

type StoreSpec struct {
	Backend  string
	Wrappers []StoreWrapperSpec
}

type StoreWrapperSpec struct {
	Type          string
	FlushInterval time.Duration
}

type RecordsSpec struct {
	Engine string
}

type StoreFactory func(filename string) (stores.Store, error)

type StoreWrapperFactory func(stores.Store, StoreWrapperSpec) (stores.Store, error)

type RecordsFactory func() records.Records[Record]

var storeFactories = map[string]StoreFactory{
	StoreBackendDisk:  func(filename string) (stores.Store, error) { return stores.NewStoreDisk(filename) },
	StoreBackendJSON:  func(filename string) (stores.Store, error) { return stores.NewStoreJson(filename) },
	StoreBackendCrazy: func(filename string) (stores.Store, error) { return stores.NewStoreCrazy(filename) },
}

var storeWrapperFactories = map[string]StoreWrapperFactory{
	StoreWrapperSnappy: func(store stores.Store, _ StoreWrapperSpec) (stores.Store, error) {
		return stores.NewStoreSnappy(store), nil
	},
	StoreWrapperAsync: func(store stores.Store, _ StoreWrapperSpec) (stores.Store, error) {
		return stores.NewStoreAsync(store), nil
	},
	StoreWrapperFlusher: func(store stores.Store, spec StoreWrapperSpec) (stores.Store, error) {
		interval := spec.FlushInterval
		if interval == 0 {
			interval = 10 * time.Second
		}
		return stores.NewStoreFlusher(store, interval), nil
	},
}

var recordsFactories = map[string]RecordsFactory{
	RecordsCorrect: func() records.Records[Record] { return records.NewRecordsCorrect[Record]() },
	RecordsFast:    func() records.Records[Record] { return records.NewRecordsFast[Record]() },
	RecordsHyper:   func() records.Records[Record] { return records.NewRecordsHyper[Record]() },
	"Hyper":        func() records.Records[Record] { return records.NewRecordsHyper[Record]() },
	RecordsTurbo:   func() records.Records[Record] { return records.NewRecordsTurbo[Record]() },
	"Turbo":        func() records.Records[Record] { return records.NewRecordsTurbo[Record]() },
	RecordsUltra:   func() records.Records[Record] { return records.NewRecordsUltra[Record]() },
}

func DefaultCollectionSpec(filename string) CollectionSpec {
	return CollectionSpec{
		Name:     filepath.Base(filename),
		Filename: filename,
		Store: StoreSpec{
			Backend: StoreBackendDisk,
			Wrappers: []StoreWrapperSpec{
				{Type: StoreWrapperAsync},
			},
		},
		Records: RecordsSpec{Engine: RecordsUltra},
	}
}

func OpenCollectionSpec(spec CollectionSpec) (*Collection, error) {
	if spec.Filename == "" {
		return nil, fmt.Errorf("collection filename is required")
	}
	if spec.Name == "" {
		spec.Name = filepath.Base(spec.Filename)
	}
	if spec.Store.Backend == "" {
		spec.Store.Backend = StoreBackendDisk
	}
	if spec.Records.Engine == "" {
		spec.Records.Engine = RecordsUltra
	}

	storeFactory, ok := storeFactories[spec.Store.Backend]
	if !ok {
		return nil, fmt.Errorf("unknown raw store type: %s", spec.Store.Backend)
	}
	store, err := storeFactory(spec.Filename)
	if err != nil {
		return nil, err
	}

	for _, wrapper := range spec.Store.Wrappers {
		if wrapper.Type == "" {
			continue
		}
		wrapperFactory, ok := storeWrapperFactories[wrapper.Type]
		if !ok {
			_ = store.Close()
			return nil, fmt.Errorf("unknown store wrapper type: %s", wrapper.Type)
		}
		store, err = wrapperFactory(store, wrapper)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
	}

	recordsFactory, ok := recordsFactories[spec.Records.Engine]
	if !ok {
		_ = store.Close()
		return nil, fmt.Errorf("unknown records type: %s", spec.Records.Engine)
	}

	col := NewCollectionConfigured(spec.Name, store, recordsFactory(), recordsFactory)
	col.SetFilepath(spec.Filename)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}
