package collectionv4

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	"github.com/tidwall/gjson"
)

func TestAll(t *testing.T) {

	filename := path.Join(t.TempDir(), "data.wal")

	{
		store, _ := NewJournal(filename)
		col := NewCollection("users", store)

		stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)

		// Insertar
		col.Insert([]byte(`{"name": "Alice"}`))
		col.Insert([]byte(`{"name": "Bob"}`))
		col.Delete(0) // Borra a Alice

		// Iterar (solo debería imprimir a Bob)
		col.Traverse(func(id int64, data []byte) bool {
			fmt.Printf("ID: %d, Data: %s\n", id, string(data))
			return true
		})

		close(stopFlusher) // Detiene la goroutine
		store.Close()      // Vacía el último buffer y cierra el archivo
	}

	{
		store, _ := NewJournal(filename)
		col := NewCollection("users", store)

		// ¡Recuperamos el estado desde disco!
		if err := col.Recover(); err != nil {
			// Aquí decides qué hacer si el WAL está corrupto.
			// En sistemas serios, se trunca el WAL hasta el último punto sano.
			fmt.Printf("Atención, error al arrancar: %v\n", err)
		}

		// Iterar (solo debería imprimir a Bob)
		col.Traverse(func(id int64, data []byte) bool {
			fmt.Printf("ID: %d, Data: %s\n", id, string(data))
			return true
		})

		store.Close() // Vacía el último buffer y cierra el archivo
	}

}

func TestRecoveryPerformance(t *testing.T) {
	filename := path.Join(t.TempDir(), "perf_data.wal")
	const numDocs = 10_000_000

	// Usamos un payload realista pero fijo para no medir el tiempo de generación de strings
	payload := []byte(`{"name": "Test User", "email": "test@example.com", "active": true, "balance": 1500.50}`)

	// ==========================================
	// FASE 1: Inserción Masiva
	// ==========================================
	{
		store, err := NewJournal(filename)
		if err != nil {
			t.Fatalf("Error creando store: %v", err)
		}
		col := NewCollection("users", store)

		// Flusher en background (Estrategia C)
		stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)

		t.Logf("Iniciando inserción de %d documentos...", numDocs)
		startInsert := time.Now()

		for i := 0; i < numDocs; i++ {
			if _, err := col.Insert(payload); err != nil {
				t.Fatalf("Error en insert %d: %v", i, err)
			}
		}

		insertDuration := time.Since(startInsert)
		t.Logf("✅ Inserción completada en: %v (%.2f docs/segundo)", insertDuration, float64(numDocs)/insertDuration.Seconds())

		// Apagado limpio para asegurar que todo baje a disco
		close(stopFlusher)
		if err := store.Close(); err != nil {
			t.Fatalf("Error cerrando store: %v", err)
		}

		// Opcional: ver cuánto pesa el archivo en disco
		info, _ := os.Stat(filename)
		t.Logf("📦 Tamaño del Journal (WAL) en disco: %.2f MB", float64(info.Size())/(1024*1024))
	}

	// ==========================================
	// FASE 2: Lectura y Reconstrucción (Recover)
	// ==========================================
	{
		store, err := NewJournal(filename)
		if err != nil {
			t.Fatalf("Error abriendo store para recuperación: %v", err)
		}
		col := NewCollection("users", store)

		t.Logf("Iniciando recuperación desde disco...")
		startRecover := time.Now()

		if err := col.Recover(); err != nil {
			t.Fatalf("Error fatal recuperando datos: %v", err)
		}

		recoverDuration := time.Since(startRecover)
		t.Logf("✅ Recuperación completada en: %v (%.2f docs/segundo)", recoverDuration, float64(numDocs)/recoverDuration.Seconds())

		// ==========================================
		// FASE 3: Verificación de Integridad
		// ==========================================
		var count int
		col.Traverse(func(id int64, data []byte) bool {
			count++
			return true
		})

		if count != numDocs {
			t.Errorf("❌ Integridad fallida: Se esperaban %d documentos, se recuperaron %d", numDocs, count)
		} else {
			t.Logf("✅ Integridad verificada: %d documentos en memoria.", count)
		}

		store.Close()
	}
}

func TestJSONOperations(t *testing.T) {
	filename := path.Join(t.TempDir(), "json_data.wal")
	store, _ := NewJournal(filename)
	col := NewCollection("users", store)

	stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)
	defer close(stopFlusher)
	defer store.Close()

	id1, _ := col.Insert([]byte(`{"name": "Alice", "age": 30}`))
	id2, _ := col.Insert([]byte(`{"name": "Bob", "age": 40}`))

	// Test Get
	res, err := col.Get(id1)
	if err != nil || gjson.GetBytes(res, "name").String() != "Alice" {
		t.Fatalf("Expected Alice, got %v (err: %v)", string(res), err)
	}

	// Test Patch
	err = col.Patch(id1, "age", 31)
	if err != nil {
		t.Fatalf("Failed to patch field: %v", err)
	}
	res, _ = col.Get(id1)
	if gjson.GetBytes(res, "age").Int() != 31 {
		t.Fatalf("Expected 31, got %v", gjson.GetBytes(res, "age").Int())
	}

	// Test Filter
	count := 0
	rows := col.Filter("name", "Bob")
	for rows.Next() {
		count++
		id, _ := rows.Read()
		if id != id2 {
			t.Fatalf("Expected id2, got %v", id)
		}
	}
	if count != 1 {
		t.Fatalf("Expected 1 match, got %d", count)
	}
}

func BenchmarkJSONRead_GJSON(b *testing.B) {
	data := []byte(`{"name": "Alice", "age": 30, "active": true, "address": {"city": "Madrid"}}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = gjson.GetBytes(data, "address.city").String()
	}
}

func BenchmarkJSONRead_Unmarshal(b *testing.B) {
	data := []byte(`{"name": "Alice", "age": 30, "active": true, "address": {"city": "Madrid"}}`)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var m map[string]interface{}
		_ = json.Unmarshal(data, &m)
		if addr, ok := m["address"].(map[string]interface{}); ok {
			_ = addr["city"].(string)
		}
	}
}
