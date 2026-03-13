package collectionv4

import (
	"fmt"
)

func indexInsert(indexes map[string]Index, id int64, data []byte) (err error) {
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

	return
}

func indexRemove(indexes map[string]Index, id int64, data []byte) (err error) {
	for key, index := range indexes {
		err = index.Remove(id, data)
		if err != nil {
			return fmt.Errorf("index remove '%s': %s", key, err.Error())
		}
	}
	return
}

type CreateIndexCommand struct {
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Options interface{} `json:"options"`
}

type DropIndexCommand struct {
	Name string `json:"name"`
}
