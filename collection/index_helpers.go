package collection

import (
	"fmt"
)

func indexInsertSync(indexes map[string]Index, id int64, data []byte) (hasAsync bool, err error) {
	if len(indexes) == 0 {
		return false, nil
	}
	rollbacks := make([]Index, 0, len(indexes))

	defer func() {
		if err == nil {
			return
		}
		for _, index := range rollbacks {
			index.Remove(id, data)
		}
	}()

	for key, index := range indexes {
		if index.IsUnique() {
			err = index.Add(id, data)
			if err != nil {
				return false, fmt.Errorf("index add '%s': %s", key, err.Error())
			}
			rollbacks = append(rollbacks, index)
		} else {
			hasAsync = true
		}
	}

	return hasAsync, nil
}

func indexRemoveSync(indexes map[string]Index, id int64, data []byte) (hasAsync bool, err error) {
	for key, index := range indexes {
		if index.IsUnique() {
			err = index.Remove(id, data)
			if err != nil {
				return false, fmt.Errorf("index remove '%s': %s", key, err.Error())
			}
		} else {
			hasAsync = true
		}
	}
	return hasAsync, nil
}

// releasingIndex is implemented by unique indexes. Together with an idempotent
// Add it lets a document move to new keys without ever leaving the index.
type releasingIndex interface {
	// Release removes the keys of oldData owned by id that newData does not use.
	Release(id int64, oldData, newData []byte)
}

// indexReserveSync adds the keys of newData to every unique index while id
// keeps its current keys. On error nothing is reserved.
func indexReserveSync(indexes map[string]Index, id int64, oldData, newData []byte) (reserved []Index, hasAsync bool, err error) {
	reserved = make([]Index, 0, len(indexes))
	for key, index := range indexes {
		if !index.IsUnique() {
			hasAsync = true
			continue
		}
		if err := index.Add(id, newData); err != nil {
			indexReleaseSync(reserved, id, newData, oldData)
			return nil, false, fmt.Errorf("index add '%s': %s", key, err.Error())
		}
		reserved = append(reserved, index)
	}
	return reserved, hasAsync, nil
}

// indexReleaseSync drops the keys of fromData that toData does not use. It
// commits a reservation (old -> new) or rolls it back (new -> old).
func indexReleaseSync(reserved []Index, id int64, fromData, toData []byte) {
	for _, index := range reserved {
		if r, ok := index.(releasingIndex); ok {
			r.Release(id, fromData, toData)
			continue
		}
		index.Remove(id, fromData)
		index.Add(id, toData)
	}
}

func indexInsertFull(indexes map[string]Index, id int64, data []byte) (err error) {
	rollbacks := make([]Index, 0, len(indexes))

	defer func() {
		if err == nil {
			return
		}
		for _, index := range rollbacks {
			index.Remove(id, data)
		}
	}()

	for key, index := range indexes {
		err = index.Add(id, data)
		if err != nil {
			return fmt.Errorf("index add '%s': %s", key, err.Error())
		}
		rollbacks = append(rollbacks, index)
	}

	return nil
}

func indexRemoveFull(indexes map[string]Index, id int64, data []byte) (err error) {
	for key, index := range indexes {
		err = index.Remove(id, data)
		if err != nil {
			return fmt.Errorf("index remove '%s': %s", key, err.Error())
		}
	}
	return nil
}

type CreateIndexCommand struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

type DropIndexCommand struct {
	Name string `json:"name"`
}
