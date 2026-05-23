package fson

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/fulldump/inceptiondb/collection/stonejson"
)

var benchData = []byte(`{
    "id": 10293,
    "name": "Geometric Gemini",
    "active": true,
    "balance": 4500.67,
    "email": "ai@example.com",
    "address": "123 Silicon Valley",
    "tags": ["ai", "go", "fast"],
    "version": "1.0.2"
}`)

func BenchmarkSTD(b *testing.B) {
	b.ReportAllocs() // Esto le dice a Go que cuente la memoria
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m map[string]any
		json.Unmarshal(benchData, &m)
		_ = m["balance"]
	}
}

func BenchmarkStoneOffsets(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m, _ := stonejson.ParseToOffsets(benchData)
		_ = m.Get("balance")
	}
}

// Puedes añadir los otros dos (Flatten y STD+newobject) de la misma forma...

type ObjectSlices struct {
	Keys   []string
	Coords []stonejson.ValueCoord
}

func (o *ObjectSlices) Get(key string) uint32 {
	for i := range o.Keys {
		if o.Keys[i] == key {
			return o.Coords[i].Offset
		}
	}
	return 0
}

// Escenario B: Estructura con Mapa
type ObjectMap struct {
	Index map[string]stonejson.ValueCoord
}

func (o *ObjectMap) Get(key string) uint32 {
	if v, ok := o.Index[key]; ok {
		return v.Offset
	}
	return 0
}

func BenchmarkSearchComparison(b *testing.B) {
	// Probamos diferentes densidades de campos
	fieldCounts := []int{2, 5, 10, 20, 50, 100}

	for _, count := range fieldCounts {
		// Setup
		keys := make([]string, count)
		m := make(map[string]stonejson.ValueCoord)
		coords := make([]stonejson.ValueCoord, count)

		for i := 0; i < count; i++ {
			keys[i] = fmt.Sprintf("field_key_%d", i)
			m[keys[i]] = stonejson.ValueCoord{Offset: uint32(i)}
			coords[i] = stonejson.ValueCoord{Offset: uint32(i)}
		}

		searchKey := keys[count-1] // Buscamos siempre el último (peor caso para lineal)

		objSlice := &ObjectSlices{Keys: keys, Coords: coords}
		objMap := &ObjectMap{Index: m}

		b.Run(fmt.Sprintf("Linear-%d", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = objSlice.Get(searchKey)
			}
		})

		b.Run(fmt.Sprintf("Map-%d", count), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_ = objMap.Get(searchKey)
			}
		})
	}
}
