package utils

import (
	"sort"
)

func GetKeys[T any](m map[string]T) []string {
	keys := []string{}
	for k, _ := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
