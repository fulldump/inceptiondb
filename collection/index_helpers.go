package collection

import (
	"fmt"
)

func indexInsertSync(indexes map[string]Index, id int64, data []byte) (hasAsync bool, err error) {
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
