package utils

import (
	"encoding/json"
)

func Remarshal(input interface{}, output interface{}) (err error) {
	b, err := json.Marshal(input)
	if nil != err {
		return
	}
	return json.Unmarshal(b, output)
}
