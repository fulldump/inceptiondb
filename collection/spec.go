package collection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	// DurabilityRelaxed acknowledges a write once it is applied in memory and
	// queued for the WAL. A crash can lose the most recent writes.
	DurabilityRelaxed = "relaxed"
	// DurabilityStrong acknowledges a write only after it is fsynced to disk,
	// so it survives a process crash or a power loss.
	DurabilityStrong = "strong"
)

type CollectionSpec struct {
	Name       string      `json:"name"`
	Filename   string      `json:"-"`
	Durability string      `json:"durability,omitempty"`
	Store      StoreSpec   `json:"storage"`
	Records    RecordsSpec `json:"records"`
}

type StoreSpec struct {
	Backend  string             `json:"backend"`
	Wrappers []StoreWrapperSpec `json:"wrappers"`
}

type StoreWrapperSpec struct {
	Type          string        `json:"type"`
	FlushInterval time.Duration `json:"flush_interval"`
}

type RecordsSpec struct {
	Engine string `json:"engine"`
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

func ValidateDurability(durability string) error {
	switch durability {
	case "", DurabilityRelaxed, DurabilityStrong:
		return nil
	}
	return fmt.Errorf("unknown durability '%s', expected '%s' or '%s'", durability, DurabilityRelaxed, DurabilityStrong)
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

func CollectionSpecPath(filename string) string {
	return filename + ".collection.json"
}

func IsCollectionSpecFile(filename string) bool {
	return strings.HasSuffix(filename, ".collection.json") || strings.HasSuffix(filename, ".collection.json.tmp")
}

func SaveCollectionSpec(spec CollectionSpec) error {
	if spec.Filename == "" {
		return fmt.Errorf("collection filename is required")
	}
	if spec.Name == "" {
		spec.Name = filepath.Base(spec.Filename)
	}
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(CollectionSpecPath(spec.Filename), data)
}

// writeFileAtomic replaces filename with data so that after a crash it holds
// either the old or the new content, never a partial write.
func writeFileAtomic(filename string, data []byte) error {
	tmp := filename + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, filename); err != nil {
		os.Remove(tmp)
		return err
	}
	return stores.SyncDir(filepath.Dir(filename))
}

func LoadCollectionSpec(filename string) (CollectionSpec, bool, error) {
	data, err := os.ReadFile(CollectionSpecPath(filename))
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultCollectionSpec(filename), false, nil
		}
		return CollectionSpec{}, false, err
	}

	spec := DefaultCollectionSpec(filename)
	if err := json.Unmarshal(data, &spec); err != nil {
		return CollectionSpec{}, true, err
	}
	spec.Filename = filename
	if spec.Name == "" {
		spec.Name = filepath.Base(filename)
	}
	return spec, true, nil
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
	if err := ValidateDurability(spec.Durability); err != nil {
		return nil, err
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
	col.SetDurability(spec.Durability)

	if err := col.Recover(); err != nil {
		_ = store.Close()
		return nil, err
	}

	return col, nil
}
