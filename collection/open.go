package collection

import "time"

func OpenCollectionCustom(filename, rawStoreName, wrapStoreName, recordsName string) (*Collection, error) {
	spec := DefaultCollectionSpec(filename)
	spec.Store.Backend = rawStoreName
	spec.Store.Wrappers = nil
	if wrapStoreName != "" {
		spec.Store.Wrappers = []StoreWrapperSpec{{
			Type:          wrapStoreName,
			FlushInterval: 10 * time.Second,
		}}
	}
	if recordsName != "" {
		spec.Records.Engine = recordsName
	}

	return OpenCollectionSpec(spec)
}

func OpenCollection(filename string) (*Collection, error) {
	spec, _, err := LoadCollectionSpec(filename)
	if err != nil {
		return nil, err
	}
	return OpenCollectionSpec(spec)
}
