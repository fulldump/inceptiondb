package collectionv4

import (
	"fmt"
	"sync"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

type Collection struct {
	name    string
	store   Store
	journal *Journal
	mu      sync.RWMutex
}

func NewCollection(name string, journal *Journal) *Collection {
	return &Collection{
		name:    name,
		journal: journal,
		store:   NewFlatSlice(10000), // Inyectamos el FlatSlice por defecto
	}
}

// Insert crea un nuevo registro y lo persiste.
func (c *Collection) Insert(jsonData []byte) (int64, error) {
	return c.store.Insert(jsonData, c.journal)
}

// Delete borra un registro y lo persiste.
func (c *Collection) Delete(id int64) error {
	// Evitar borrar si no existe o lanzar error
	if _, ok := c.store.Get(id); !ok {
		return nil // Ya está borrado o no existe
	}

	// Persistir el borrado (payload vacío)
	if err := c.journal.Append(OpDelete, id, nil); err != nil {
		return fmt.Errorf("journal write failed on delete: %v", err)
	}

	// Liberar memoria para el GC
	c.store.Delete(id)

	return nil
}

// Get obtiene el JSON completo de un registro.
func (c *Collection) Get(id int64) ([]byte, error) {
	data, ok := c.store.Get(id)
	if !ok {
		return nil, fmt.Errorf("record not found")
	}
	return data, nil
}

// Patch modifica un campo específico de un registro usando sjson y escribe el cambio en el WAL.
// Utiliza OpUpdate para la persistencia.
func (c *Collection) Patch(id int64, path string, value interface{}) error {
	data, ok := c.store.Get(id)
	if !ok {
		return fmt.Errorf("record not found")
	}

	// sjson.SetBytes genera un nuevo slice de bytes con la modificación aplicada
	newData, err := sjson.SetBytesOptions(data, path, value, &sjson.Options{Optimistic: true})
	if err != nil {
		return fmt.Errorf("error patching json field: %v", err)
	}

	// Persistir la actualización en disco usando el WAL
	if err := c.journal.Append(OpUpdate, id, newData); err != nil {
		return fmt.Errorf("journal write failed on patch: %v", err)
	}

	// Confirmar en memoria
	c.store.Set(id, newData)

	return nil
}

// Traverse itera por todos los registros, ejecutando 'f'. Si 'f' devuelve false, se detiene.
func (c *Collection) Traverse(f func(id int64, data []byte) bool) {
	rows := c.store.Scan()
	defer rows.Close()

	for rows.Next() {
		id, data := rows.Read()
		if !f(id, data) {
			break
		}
	}
}

// Filter devuelve un iterador Rows personalizado que evalúa el 'path' gjson con el 'expected'
func (c *Collection) Filter(path string, expected interface{}) Rows {
	return &filterRows{
		source:         c.store.Scan(),
		filterPath:     path,
		filterExpected: expected,
	}
}

// Query es un placeholder para futuras consultas complejas (IQL, MongoDB queries, etc)
func (c *Collection) Query(q string) (Rows, error) {
	return nil, fmt.Errorf("querying not fully implemented yet")
}

// Recover lee el WAL y reconstruye el estado exacto de la base de datos
func (c *Collection) Recover() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 1. Limpiamos cualquier estado previo
	c.store.Reset()

	// 2. Función que reacciona a cada línea del Journal
	err := c.journal.Replay(func(op uint8, id int64, data []byte) error {
		switch op {
		case OpInsert, OpUpdate: // Para memoria, Update e Insert hacen lo mismo
			c.store.Set(id, data)
		case OpDelete:
			c.store.Delete(id) // Marcar inactivo
		default:
			return fmt.Errorf("operación desconocida en el WAL: %d", op)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("error recuperando datos: %v", err)
	}

	// 3. Reconstruir la pila de huecos (FreeList) en FlatSlice
	c.store.RebuildFreeList()

	fmt.Printf("Recuperación exitosa: %d registros activos, %d huecos\n",
		c.store.GetLen()-c.store.GetFreeListLen(), c.store.GetFreeListLen())

	return nil
}

// filterRows es un envoltorio que toma otro iterador y desecha los que no pasen el filtro
type filterRows struct {
	source         Rows
	filterPath     string
	filterExpected interface{}
}

func (f *filterRows) Next() bool {
	for f.source.Next() {
		_, data := f.source.Read()
		res := gjson.GetBytes(data, f.filterPath)
		if res.Value() == f.filterExpected {
			return true
		}
	}
	return false
}

func (f *filterRows) Read() (int64, []byte) {
	return f.source.Read()
}

func (f *filterRows) Close() {
	f.source.Close()
}
