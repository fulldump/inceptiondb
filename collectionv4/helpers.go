package collectionv4

import (
	"fmt"
	"time"
)

func StartBackgroundFlusher(store *Journal, interval time.Duration) chan struct{} {
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
