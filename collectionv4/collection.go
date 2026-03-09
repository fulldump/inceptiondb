package collectionv4

import (
	"fmt"
	"sync/atomic"

	"github.com/fulldump/inceptiondb/collectionv4/records"
)

// Record es la celda de nuestro FlatSlice
type Record struct {
	Data   []byte // El JSON puro
	Parsed any    // Espacio para caché del JSON parseado (Lazy)
	Active bool   // true si tiene datos, false si es un hueco
}

type Collection struct {
	name    string
	store   Store
	records records.Records[Record]
	maxID   atomic.Int64
}

func NewCollection(name string, store Store) *Collection {
	return &Collection{
		name:    name,
		store:   store,
		records: records.NewRecordsUltra[Record](),
	}
}

func (c *Collection) Insert(jsonData []byte) (int64, error) {
	// 1. Insertar en memoria (optimista)
	rec := Record{
		Data:   jsonData,
		Active: true,
	}
	id := c.records.Insert(rec)

	// Actualizamos el maxID atómicamente
	for {
		curr := c.maxID.Load()
		if id <= curr || c.maxID.CompareAndSwap(curr, id) {
			break
		}
	}

	// 2. Escribir en el Journal
	if err := c.store.Append(OpInsert, id, jsonData); err != nil {
		// Rollback si falla el journal
		c.records.Delete(id)
		return 0, fmt.Errorf("journal write failed: %v", err)
	}

	return id, nil
}

func (c *Collection) Delete(id int64) error {
	// Verificar si existe antes de persistir (opcional)
	rec := c.records.Get(id)
	if !rec.Active {
		return nil // Ya está borrado o no existe
	}

	// Persistir el borrado (payload vacío)
	if err := c.store.Append(OpDelete, id, nil); err != nil {
		return err
	}

	// Liberar memoria para el GC y marcar como inactivo
	c.records.Delete(id)

	return nil
}

// Recover lee el WAL y reconstruye el estado exacto de la base de datos
func (c *Collection) Recover() error {
	// 1. Limpiamos cualquier estado previo
	c.records = records.NewRecordsUltra[Record]()
	c.maxID.Store(0)

	var localMaxID int64 = 0

	// 2. Función que reacciona a cada línea del Journal
	err := c.store.Replay(func(op uint8, id int64, data []byte) error {
		if id > localMaxID {
			localMaxID = id
		}

		switch op {
		case OpInsert, OpUpdate: // Para memoria, Update e Insert hacen lo mismo
			c.records.Set(id, Record{
				Data:   data,
				Active: true,
			})
		case OpDelete:
			c.records.Delete(id)
		default:
			return fmt.Errorf("operación desconocida en el WAL: %d", op)
		}

		return nil
	})

	c.maxID.Store(localMaxID)

	if err != nil {
		return fmt.Errorf("error recuperando datos: %v", err)
	}

	fmt.Printf("Recuperación exitosa: maxID = %d\n", localMaxID)

	return nil
}
