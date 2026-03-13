package collectionv4

import (
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fulldump/inceptiondb/collectionv4/records"
	"github.com/google/uuid"
)

// Record es la celda de nuestro FlatSlice
type Record struct {
	Data   []byte // El JSON puro
	Parsed any    // Espacio para caché del JSON parseado (Lazy)
	Active bool   // true si tiene datos, false si es un hueco
}

type Collection struct {
	name     string
	store    Store
	records  records.Records[Record]
	maxID    atomic.Int64
	indexes  map[string]Index
	defaults map[string]any
	mu       sync.RWMutex
}

func NewCollection(name string, store Store) *Collection {
	return &Collection{
		name:     name,
		store:    store,
		records:  records.NewRecordsUltra[Record](),
		indexes:  map[string]Index{},
		defaults: nil,
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

	c.mu.RLock()
	err := indexInsert(c.indexes, id, jsonData)
	c.mu.RUnlock()
	if err != nil {
		c.records.Delete(id)
		return 0, err
	}

	// 2. Escribir en el Journal
	if err := c.store.Append(OpInsert, id, jsonData); err != nil {
		// Rollback si falla el journal
		c.mu.RLock()
		indexRemove(c.indexes, id, jsonData)
		c.mu.RUnlock()
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

	c.mu.RLock()
	err := indexRemove(c.indexes, id, rec.Data)
	c.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("could not free index: %w", err)
	}

	// Persistir el borrado (payload vacío)
	if err := c.store.Append(OpDelete, id, nil); err != nil {
		// Si el log falla, tenemos que deshacer el indexRemove, pero es complejo.
		// Al menos devolvemos error
		return err
	}

	// Liberar memoria para el GC y marcar como inactivo
	c.records.Delete(id)

	return nil
}

// Recover lee el WAL y reconstruye el estado exacto de la base de datos
func (c *Collection) Recover() error { // nolint:gocyclo
	// 1. Limpiamos cualquier estado previo
	c.records = records.NewRecordsUltra[Record]()
	c.indexes = map[string]Index{}
	c.maxID.Store(0)

	var localMaxID int64 = 0

	// 2. Función que reacciona a cada línea del Journal
	err := c.store.Replay(func(op uint8, id int64, data []byte) error {
		if id > localMaxID {
			localMaxID = id
		}

		switch op {
		case OpInsert, OpUpdate:
			// Si es un update, comprobamos si ya había un dato anterior para limpiar los índices
			rec := c.records.Get(id)
			if rec.Active {
				indexRemove(c.indexes, id, rec.Data)
			}

			c.records.Set(id, Record{
				Data:   data,
				Active: true,
			})

			indexInsert(c.indexes, id, data)

		case OpDelete:
			rec := c.records.Get(id)
			if rec.Active {
				indexRemove(c.indexes, id, rec.Data)
			}
			c.records.Delete(id)

		case OpCreateIndex:
			cmd := &CreateIndexCommand{}
			if err := json.Unmarshal(data, cmd); err != nil {
				return err
			}

			index, err := newIndexFromCreateCommand(cmd)
			if err != nil {
				return err
			}

			c.indexes[cmd.Name] = index
			for i := int64(0); i <= localMaxID; i++ {
				rec := c.records.Get(i)
				if rec.Active {
					if err := index.Add(i, rec.Data); err != nil {
						return fmt.Errorf("error indexing existing data: %w", err)
					}
				}
			}

		case OpDropIndex:
			cmd := &DropIndexCommand{}
			if err := json.Unmarshal(data, cmd); err == nil {
				delete(c.indexes, cmd.Name)
			}

		case OpSetDefaults:
			var defaults map[string]any
			if err := json.Unmarshal(data, &defaults); err == nil {
				c.defaults = defaults
			}

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

func (c *Collection) CreateIndex(name string, options interface{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indexes[name]; exists {
		return fmt.Errorf("index '%s' already exists", name)
	}

	var index Index
	var typeStr string

	switch value := options.(type) {
	case *IndexMapOptions:
		typeStr = "map"
		index = NewIndexMap(value)
	case *IndexBTreeOptions:
		typeStr = "btree"
		index = NewIndexBTree(value)
	case *IndexFTSOptions:
		typeStr = "fts"
		index = NewIndexFTS(value)
	default:
		return fmt.Errorf("unexpected options parameters, it should be [*IndexMapOptions|*IndexBTreeOptions|*IndexFTSOptions]")
	}

	c.indexes[name] = index

	// Llenar el índice con los datos existentes
	maxID := c.maxID.Load()
	for i := int64(0); i <= maxID; i++ {
		rec := c.records.Get(i)
		if !rec.Active {
			continue
		}
		if err := index.Add(i, rec.Data); err != nil {
			// En caso de error, podríamos hacer rollback borbrando el index de c.indexes.
			// Pero por ahora, devolvemos el error y lo removemos.
			delete(c.indexes, name)
			return fmt.Errorf("error indexing existing data: %w", err)
		}
	}

	// Persistir la creación
	payload, err := json.Marshal(&CreateIndexCommand{
		Name:    name,
		Type:    typeStr,
		Options: options,
	})
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	return c.store.Append(OpCreateIndex, 0, payload)
}

func (c *Collection) Index(name string, options interface{}) error {
	return c.CreateIndex(name, options)
}

func (c *Collection) DropIndex(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.indexes[name]; !exists {
		return fmt.Errorf("dropIndex: index '%s' not found", name)
	}

	delete(c.indexes, name)

	payload, err := json.Marshal(&DropIndexCommand{
		Name: name,
	})
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	return c.store.Append(OpDropIndex, 0, payload)
}

func (c *Collection) TraverseIndex(name string, options []byte, f func(id int64, data []byte) bool) error {
	c.mu.RLock()
	index, exists := c.indexes[name]
	c.mu.RUnlock()
	if !exists {
		return fmt.Errorf("index '%s' not found", name)
	}

	index.Traverse(options, f)
	return nil
}

func newIndexFromCreateCommand(cmd *CreateIndexCommand) (Index, error) {
	if cmd == nil {
		return nil, fmt.Errorf("nil create index command")
	}

	if cmd.Options == nil {
		return nil, fmt.Errorf("index '%s' has nil options", cmd.Name)
	}

	optionsData, err := json.Marshal(cmd.Options)
	if err != nil {
		return nil, fmt.Errorf("marshal index options: %w", err)
	}

	switch cmd.Type {
	case "map":
		options := &IndexMapOptions{}
		if err := json.Unmarshal(optionsData, options); err != nil {
			return nil, fmt.Errorf("decode map index options: %w", err)
		}
		return NewIndexMap(options), nil
	case "btree":
		options := &IndexBTreeOptions{}
		if err := json.Unmarshal(optionsData, options); err != nil {
			return nil, fmt.Errorf("decode btree index options: %w", err)
		}
		return NewIndexBTree(options), nil
	case "fts":
		options := &IndexFTSOptions{}
		if err := json.Unmarshal(optionsData, options); err != nil {
			return nil, fmt.Errorf("decode fts index options: %w", err)
		}
		return NewIndexFTS(options), nil
	default:
		return nil, fmt.Errorf("unexpected index type '%s'", cmd.Type)
	}
}

func (c *Collection) FindOne(data interface{}) error { // nolint:gocyclo
	// Just get the first one
	rows := c.Scan()
	if rows.Next() {
		_, payload := rows.Read()
		return json.Unmarshal(payload, data)
	}
	return fmt.Errorf("collection is empty")
}

func (c *Collection) Traverse(f func(data []byte)) {
	rows := c.Scan()
	for rows.Next() {
		_, payload := rows.Read()
		f(payload)
	}
}

func (c *Collection) TraverseRange(from, to int, f func(data []byte)) {
	count := 0
	rows := c.Scan()
	for rows.Next() {
		if count >= to && to > 0 {
			break
		}
		if count >= from {
			_, payload := rows.Read()
			f(payload)
		}
		count++
	}
}

func (c *Collection) SetDefaults(defaults map[string]any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.defaults = defaults

	payload, err := json.Marshal(defaults)
	if err != nil {
		return fmt.Errorf("json encode payload: %w", err)
	}

	return c.store.Append(OpSetDefaults, 0, payload)
}

func (c *Collection) InsertMap(item map[string]any) (int64, error) {
	c.mu.RLock()
	defs := c.defaults
	c.mu.RUnlock()

	for k, v := range defs {
		if item[k] != nil {
			continue
		}
		switch v {
		case "uuid()":
			item[k] = uuid.NewString()
		case "unixnano()":
			item[k] = time.Now().UnixNano()
		default:
			item[k] = v
		}
	}

	payload, err := json.Marshal(item)
	if err != nil {
		return 0, fmt.Errorf("json encode payload: %w", err)
	}

	return c.Insert(payload)
}
