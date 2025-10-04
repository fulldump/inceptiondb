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

func RemarshalMap(input any) (output map[string]any) {
	b, err := json.Marshal(input)
	if err != nil {
		panic("RemarshalMap failed to marshal input")
	}
	err = json.Unmarshal(b, &output)
	if err != nil {
		panic("RemarshalMap failed to unmarshal input")
	}
	return
}
