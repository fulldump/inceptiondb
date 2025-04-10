package collection

import (
	"fmt"
	"testing"
)

// createPayload genera un JSON de ejemplo con la clave dada.
func createPayload(key, field string) []byte {
	return []byte(fmt.Sprintf(`{"%s": "%s"}`, field, key))
}

//
// Benchmark para la operación AddRow
//

func BenchmarkIndexMap_AddRow(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexMap(options)

	// Preparamos un slice de payloads únicos para evitar conflictos por clave duplicada.
	payloads := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		payloads[i] = createPayload(key, options.Field)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := &Row{Payload: payloads[i]}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}
}

func BenchmarkIndexSyncMap_AddRow(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexSyncMap(options)

	payloads := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		payloads[i] = createPayload(key, options.Field)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := &Row{Payload: payloads[i]}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}
}

//
// Benchmark para la operación RemoveRow
//

func BenchmarkIndexMap_RemoveRow(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexMap(options)

	// Prellenamos el índice con b.N filas.
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		row := &Row{Payload: createPayload(key, options.Field)}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}

	// Preparamos los payloads para eliminar.
	payloads := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		payloads[i] = createPayload(key, options.Field)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := &Row{Payload: payloads[i]}
		if err := index.RemoveRow(row); err != nil {
			b.Fatalf("RemoveRow error: %v", err)
		}
	}
}

func BenchmarkIndexSyncMap_RemoveRow(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexSyncMap(options)

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		row := &Row{Payload: createPayload(key, options.Field)}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}

	payloads := make([][]byte, b.N)
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key-%d", i)
		payloads[i] = createPayload(key, options.Field)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		row := &Row{Payload: payloads[i]}
		if err := index.RemoveRow(row); err != nil {
			b.Fatalf("RemoveRow error: %v", err)
		}
	}
}

//
// Benchmark para la operación Traverse
//

func BenchmarkIndexMap_Traverse(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexMap(options)

	// Prellenamos el índice con un número razonable de filas.
	numRows := 10000
	for i := 0; i < numRows; i++ {
		key := fmt.Sprintf("key-%d", i)
		row := &Row{Payload: createPayload(key, options.Field)}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}

	// Usamos opciones para recorrer la clave "key-0".
	traverseOptions := []byte(`{"value": "key-0"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Traverse(traverseOptions, func(row *Row) bool {
			return true
		})
	}
}

func BenchmarkIndexSyncMap_Traverse(b *testing.B) {
	options := &IndexMapOptions{Field: "key", Sparse: false}
	index := NewIndexSyncMap(options)

	numRows := 10000
	for i := 0; i < numRows; i++ {
		key := fmt.Sprintf("key-%d", i)
		row := &Row{Payload: createPayload(key, options.Field)}
		if err := index.AddRow(row); err != nil {
			b.Fatalf("AddRow error: %v", err)
		}
	}

	traverseOptions := []byte(`{"value": "key-0"}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		index.Traverse(traverseOptions, func(row *Row) bool {
			return true
		})
	}
}
