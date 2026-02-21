package main

import (
	"fmt"
	"time"

	"github.com/fulldump/inceptiondb/collectionv4"
)

func main() {
	store, _ := collectionv4.NewStore("data.wal")
	col := collectionv4.NewCollection("users", store)

	stopFlusher := StartBackgroundFlusher(store, 500*time.Millisecond)

	// Insertar
	// col.Insert([]byte(`{"name": "Alice"}`))
	// col.Insert([]byte(`{"name": "Bob"}`))
	// col.Delete(0) // Borra a Alice

	// Iterar (solo debería imprimir a Bob)
	rows := col.Scan()
	for rows.Next() {
		id, data := rows.Read()
		fmt.Printf("ID: %d, Data: %s\n", id, string(data))
	}

	close(stopFlusher) // Detiene la goroutine
	store.Close()      // Vacía el último buffer y cierra el archivo
}

func StartBackgroundFlusher(store *collectionv4.Store, interval time.Duration) chan struct{} {
	stopChan := make(chan struct{})

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Cada X milisegundos bajamos a disco
				if err := store.Sync(); err != nil {
					fmt.Printf("Error haciendo sync del WAL: %v\n", err)
				}
			case <-stopChan:
				// Señal para detener el flusher al apagar
				return
			}
		}
	}()

	return stopChan
}
