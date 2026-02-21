package collectionv4

import (
	"sync"
)

// Record es la celda de nuestro FlatSlice
type Record struct {
	Data   []byte // El JSON puro
	Parsed any    // Espacio para caché del JSON parseado (Lazy)
	Active bool   // true si tiene datos, false si es un hueco
}

// FlatSlice es un almacén en memoria hiper-rápido basado en arrays
type FlatSlice struct {
	records  []Record // Nuestro array contiguo
	freeList []int64  // Pila de IDs borrados
	mu       sync.RWMutex
}

func NewFlatSlice(capacity int) *FlatSlice {
	return &FlatSlice{
		records:  make([]Record, 0, capacity),
		freeList: make([]int64, 0),
	}
}

func (fs *FlatSlice) AllocID() int64 {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var id int64
	// Obtener ID (reutilizar o crecer)
	if n := len(fs.freeList); n > 0 {
		id = fs.freeList[n-1] // Pop
		fs.freeList = fs.freeList[:n-1]
	} else {
		id = int64(len(fs.records))
		fs.records = append(fs.records, Record{}) // Crecer el slice
	}
	return id
}

func (fs *FlatSlice) RollbackID(id int64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.freeList = append(fs.freeList, id)
}

func (fs *FlatSlice) Insert(data []byte, journal *Journal) (int64, error) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var id int64
	// Obtener ID (reutilizar o crecer)
	if n := len(fs.freeList); n > 0 {
		id = fs.freeList[n-1] // Pop
		fs.freeList = fs.freeList[:n-1]
	} else {
		id = int64(len(fs.records))
		fs.records = append(fs.records, Record{}) // Crecer el slice
	}

	// Ejecutar append sobre el journal bajo el lock atómico sin crear closures
	if err := journal.Append(OpInsert, id, data); err != nil {
		fs.freeList = append(fs.freeList, id) // Rollback
		return 0, err
	}

	fs.records[id] = Record{
		Data:   data,
		Active: true,
	}

	return id, nil
}

func (fs *FlatSlice) Set(id int64, data []byte) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Si el ID es mayor que el tamaño actual del slice, debemos expandirlo.
	// Esto pasa en la recuperación reconstruyendo el array original.
	for int64(len(fs.records)) <= id {
		fs.records = append(fs.records, Record{Active: false})
	}

	fs.records[id] = Record{
		Data:   data,
		Active: true,
	}
}

func (fs *FlatSlice) Delete(id int64) {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	if id < 0 || id >= int64(len(fs.records)) || !fs.records[id].Active {
		return // Ya está borrado o no existe
	}

	// Liberar memoria para el GC y marcar como inactivo
	fs.records[id] = Record{Active: false}
	fs.freeList = append(fs.freeList, id) // Push
}

func (fs *FlatSlice) Get(id int64) ([]byte, bool) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	if id < 0 || id >= int64(len(fs.records)) || !fs.records[id].Active {
		return nil, false
	}
	return fs.records[id].Data, true
}

func (fs *FlatSlice) Scan() Rows {
	return &flatSliceRows{
		fs:    fs,
		index: -1,
	}
}

func (fs *FlatSlice) Reset() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.records = make([]Record, 0)
	fs.freeList = make([]int64, 0)
}

// Genera la pila de huecos basándose en el estado actual de los records (usado en Recover)
func (fs *FlatSlice) RebuildFreeList() {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	fs.freeList = make([]int64, 0)
	for i := int64(len(fs.records) - 1); i >= 0; i-- {
		if !fs.records[i].Active {
			fs.freeList = append(fs.freeList, i)
		}
	}
}

func (fs *FlatSlice) GetLen() int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return len(fs.records)
}

func (fs *FlatSlice) GetFreeListLen() int {
	fs.mu.RLock()
	defer fs.mu.RUnlock()
	return len(fs.freeList)
}

// flatSliceRows es el iterador interno
type flatSliceRows struct {
	fs          *FlatSlice
	index       int64
	currentID   int64
	currentData []byte
}

func (r *flatSliceRows) Next() bool {
	r.fs.mu.RLock()
	defer r.fs.mu.RUnlock()

	for {
		r.index++
		if r.index >= int64(len(r.fs.records)) {
			return false // Fin de la tabla
		}

		if r.fs.records[r.index].Active {
			r.currentID = r.index
			r.currentData = r.fs.records[r.index].Data
			return true
		}
	}
}

func (r *flatSliceRows) Read() (int64, []byte) {
	return r.currentID, r.currentData
}

func (r *flatSliceRows) Close() {
	// No op para flatSlice
}
