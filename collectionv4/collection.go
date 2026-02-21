package collectionv4

import (
	"fmt"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// Record es la celda de nuestro FlatSlice
type Record struct {
	Data   []byte // El JSON puro
	Parsed any    // Espacio para caché del JSON parseado (Lazy)
	Active bool   // true si tiene datos, false si es un hueco
}

type Collection struct {
	name     string
	store    *Store
	records  []Record // Nuestro Flat Slice
	freeList []int64  // Pila de IDs borrados
	mu       sync.RWMutex
}

func NewCollection(name string, store *Store) *Collection {
	return &Collection{
		name:     name,
		store:    store,
		records:  make([]Record, 0, 10000), // Pre-asignar capacidad
		freeList: make([]int64, 0),
	}
}

func (c *Collection) Insert(jsonData []byte) (int64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var id int64

	// 1. Obtener ID (reutilizar o crecer)
	if n := len(c.freeList); n > 0 {
		id = c.freeList[n-1] // Pop
		c.freeList = c.freeList[:n-1]
	} else {
		id = int64(len(c.records))
		c.records = append(c.records, Record{}) // Crecer el slice
	}

	// 2. Escribir en el Journal ANTES de confirmar en memoria (Durabilidad)
	if err := c.store.Append(OpInsert, id, jsonData); err != nil {
		// Rollback de la freelist en caso de fallo crítico de disco
		c.freeList = append(c.freeList, id)
		return 0, fmt.Errorf("journal write failed: %v", err)
	}

	// 3. Confirmar en memoria
	c.records[id] = Record{
		Data:   jsonData,
		Active: true,
	}

	return id, nil
}

func (c *Collection) Delete(id int64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if id < 0 || id >= int64(len(c.records)) || !c.records[id].Active {
		return nil // Ya está borrado o no existe
	}

	// Persistir el borrado (payload vacío)
	if err := c.store.Append(OpDelete, id, nil); err != nil {
		return err
	}

	// Liberar memoria para el GC y marcar como inactivo
	c.records[id] = Record{Active: false}
	c.freeList = append(c.freeList, id) // Push

	return nil
}

// Recover lee el WAL y reconstruye el estado exacto de la base de datos
func (c *Collection) Recover() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Limpiamos cualquier estado previo
	c.records = make([]Record, 0)
	c.freeList = make([]int64, 0)

	// 2. Función que reacciona a cada línea del Journal
	err := c.store.Replay(func(op uint8, id int64, data []byte) error {

		// Si el ID es mayor que el tamaño actual del slice, debemos expandirlo.
		// Esto pasa porque estamos reconstruyendo el array original.
		for int64(len(c.records)) <= id {
			c.records = append(c.records, Record{Active: false})
		}

		switch op {
		case OpInsert, OpUpdate: // Para memoria, Update e Insert hacen lo mismo
			c.records[id] = Record{
				Data:   data,
				Active: true,
			}
		case OpDelete:
			// Marcamos como inactivo y liberamos el JSON de la RAM
			c.records[id] = Record{
				Data:   nil,
				Active: false,
			}
		default:
			return fmt.Errorf("operación desconocida en el WAL: %d", op)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("error recuperando datos: %v", err)
	}

	// 3. ¡El truco de la FreeList!
	// Una vez recuperado todo el FlatSlice, lo recorremos linealmente
	// para encontrar los huecos y regenerar la pila de IDs disponibles.
	for i := int64(len(c.records) - 1); i >= 0; i-- {
		if !c.records[i].Active {
			c.freeList = append(c.freeList, i)
		}
	}

	fmt.Printf("Recuperación exitosa: %d registros activos, %d huecos en FreeList\n",
		int64(len(c.records))-int64(len(c.freeList)), len(c.freeList))

	return nil
}

// Get extrae un valor del JSON de un registro específico usando su ID y un "path" sin alocar memoria.
func (c *Collection) Get(id int64, path string) (gjson.Result, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if id < 0 || id >= int64(len(c.records)) || !c.records[id].Active {
		return gjson.Result{}, fmt.Errorf("record not found")
	}

	return gjson.GetBytes(c.records[id].Data, path), nil
}

// UpdateField modifica un campo específico de un registro usando sjson y escribe el cambio en el WAL.
// Utiliza OpUpdate para la persistencia.
func (c *Collection) UpdateField(id int64, path string, value interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if id < 0 || id >= int64(len(c.records)) || !c.records[id].Active {
		return fmt.Errorf("record not found")
	}

	// sjson.SetBytes genera un nuevo slice de bytes con la modificación aplicada
	newData, err := sjson.SetBytesOptions(c.records[id].Data, path, value, &sjson.Options{Optimistic: true})
	if err != nil {
		return fmt.Errorf("error updating json field: %v", err)
	}

	// Persistir la actualización en disco usando el WAL
	if err := c.store.Append(OpUpdate, id, newData); err != nil {
		return fmt.Errorf("journal write failed on update: %v", err)
	}

	// Confirmar en memoria
	c.records[id].Data = newData

	return nil
}
