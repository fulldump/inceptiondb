package collectionv2

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
)

func BenchmarkCollectionV2_IndexMap_AddRow(b *testing.B) {
	idx := NewIndexMap(&IndexMapOptions{Field: "id"})

	numRows := 1000000
	rows := make([]*Row, numRows)
	for i := 0; i < numRows; i++ {
		payload := fmt.Sprintf(`{"id":"key-%d","value":"data"}`, i)
		rows[i] = &Row{
			I:       i,
			Payload: json.RawMessage(payload),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var counter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1) % int64(numRows)
			err := idx.AddRow(rows[i])
			if err != nil && !strings.Contains(err.Error(), "index conflict") {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCollectionV2_IndexPK_AddRow(b *testing.B) {
	idx := NewIndexPK(&IndexPKOptions{Paths: [][]string{{"id"}}})

	numRows := 1000000
	rows := make([]*Row, numRows)
	for i := 0; i < numRows; i++ {
		payload := fmt.Sprintf(`{"id":"key-%d","value":"data"}`, i)
		rows[i] = &Row{
			I:       i,
			Payload: json.RawMessage(payload),
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	var counter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1) % int64(numRows)
			err := idx.AddRow(rows[i])
			if err != nil && !strings.HasPrefix(err.Error(), "duplicate primary key") {
				b.Fatal(err)
			}
		}
	})
}

func BenchmarkCollectionV2_IndexMap_Traverse(b *testing.B) {
	idx := NewIndexMap(&IndexMapOptions{Field: "id"})

	numRows := 100000
	keys := make([][]byte, numRows)
	for i := 0; i < numRows; i++ {
		keyStr := fmt.Sprintf("key-%d", i)
		keys[i] = []byte(fmt.Sprintf(`{"value":"%s"}`, keyStr))
		payload := fmt.Sprintf(`{"id":"%s"}`, keyStr)
		_ = idx.AddRow(&Row{I: i, Payload: json.RawMessage(payload)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	var counter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1) % int64(numRows)
			idx.Traverse(keys[i], func(r *Row) bool {
				return true
			})
		}
	})
}

func BenchmarkCollectionV2_IndexPK_Traverse(b *testing.B) {
	idx := NewIndexPK(&IndexPKOptions{Paths: [][]string{{"id"}}})

	numRows := 100000
	keys := make([][]byte, numRows)
	for i := 0; i < numRows; i++ {
		keyStr := fmt.Sprintf("key-%d", i)
		keys[i] = []byte(keyStr) // raw PK lookup
		payload := fmt.Sprintf(`{"id":"%s"}`, keyStr)
		_ = idx.AddRow(&Row{I: i, Payload: json.RawMessage(payload)})
	}

	b.ResetTimer()
	b.ReportAllocs()

	var counter int64

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := atomic.AddInt64(&counter, 1) % int64(numRows)
			idx.Traverse(keys[i], func(r *Row) bool {
				return true
			})
		}
	})
}
