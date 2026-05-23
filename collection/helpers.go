package collection

import (
	"fmt"
	"time"

	"github.com/fulldump/inceptiondb/collection/stores"
)

func StartBackgroundFlusher(store stores.Store, interval time.Duration) chan struct{} {
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
