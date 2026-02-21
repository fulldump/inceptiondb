package collectionv4

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"
)

func TestAll(t *testing.T) {

	filename := path.Join(t.TempDir(), "data.wal")

	{
		store, _ := NewStore(filename)
		col := NewCollection("users", store)

		stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)

		// Insertar
		col.Insert([]byte(`{"name": "Alice"}`))
		col.Insert([]byte(`{"name": "Bob"}`))
		col.Delete(0) // Borra a Alice

		// Iterar (solo debería imprimir a Bob)
		rows := col.Scan()
		for rows.Next() {
			id, data := rows.Read()
			fmt.Printf("ID: %d, Data: %s\n", id, string(data))
		}

		close(stopFlusher) // Detiene la goroutine
		store.Close()      // Vacía el último buffer y cierra el archivo
	}

	{
		store, _ := NewStore(filename)
		col := NewCollection("users", store)

		// ¡Recuperamos el estado desde disco!
		if err := col.Recover(); err != nil {
			// Aquí decides qué hacer si el WAL está corrupto.
			// En sistemas serios, se trunca el WAL hasta el último punto sano.
			fmt.Printf("Atención, error al arrancar: %v\n", err)
		}

		// Iterar (solo debería imprimir a Bob)
		rows := col.Scan()
		for rows.Next() {
			id, data := rows.Read()
			fmt.Printf("ID: %d, Data: %s\n", id, string(data))
		}

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
		store, err := NewStore(filename)
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
		store, err := NewStore(filename)
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
		rows := col.Scan()
		for rows.Next() {
			count++
		}

		if count != numDocs {
			t.Errorf("❌ Integridad fallida: Se esperaban %d documentos, se recuperaron %d", numDocs, count)
		} else {
			t.Logf("✅ Integridad verificada: %d documentos en memoria.", count)
		}

		store.Close()
	}
}
