package fson

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/fulldump/inceptiondb/collection/stonejson"
)

func TestLolo(t *testing.T) {

	data := []byte(`{
	"id": 10293,
	"name": "Geometric Gemini",
	"active": true,
	"balance": 4500.67,
	"email": "ai@example.com",
	"address": "123 Silicon Valley",
	"tags": ["ai", "go", "fast"],
	"version": "1.0.2"
}`)

	N := 100000

	t0 := time.Now()
	for range N {
		var m map[string]any
		json.Unmarshal(data, &m)
		value, ok := m["balance"]
		if !ok {
			t.Error("missing balance")
		}
		_ = value
	}
	fmt.Println("STD", time.Since(t0))

	t2 := time.Now()
	for range N {
		var m ObjectJSON
		json.Unmarshal(data, &m)
		value := m.Get("balance")
		_ = value
	}
	fmt.Println("STD + newobject", time.Since(t2))

	t1 := time.Now()
	for range N {
		m, _ := FlattenJSON(data)
		value := m.Get("balance")
		_ = value
	}
	fmt.Println("Flatten", time.Since(t1))

	t3 := time.Now()
	for range N {
		m, _ := stonejson.ParseToOffsets(data)
		value := m.Get("balance")
		_ = value
	}
	fmt.Println("Stone", time.Since(t3))

}

func TestCorrectness(t *testing.T) {

	data := []byte(`{
	"id": 10293,
	"name": "Geometric Gemini",
	"active": true,
	"balance": 4500.67,
	"email": "ai@\nexample.com",
	"address": "123 Silicon Valley",
	"tags": ["ai", "go", "fast"],
	"version": "1.0.2"
}`)

	m, _ := stonejson.ParseToOffsets(data)
	value := m.Get("tags")
	fmt.Println(value)

}
