package collectionv4

import (
	"github.com/tidwall/gjson"
)

type Rows struct {
	col   *Collection
	index int64
	// variables para el registro actual
	currentID   int64
	currentData []byte

	// Filtros opcionales
	hasFilter      bool
	filterPath     string
	filterExpected interface{}
}

func (c *Collection) Scan() *Rows {
	// Ojo: En un entorno altamente concurrente, deberías tomar un Read Lock
	// o usar un snapshot para evitar que los datos cambien mientras iteras.
	return &Rows{
		col:   c,
		index: -1,
	}
}

// Where añade un filtro básico al iterador actual.
// Por ahora soporta igualdad estricta con el valor JSON.
func (r *Rows) Where(path string, expected interface{}) *Rows {
	r.hasFilter = true
	r.filterPath = path
	r.filterExpected = expected
	return r
}

// Next avanza al siguiente registro válido (saltando huecos y aplicando filtros si existen).
// Devuelve false cuando no hay más registros.
func (r *Rows) Next() bool {
	r.col.mu.RLock()
	defer r.col.mu.RUnlock()

	for {
		r.index++
		if r.index >= int64(len(r.col.records)) {
			return false // Fin de la tabla
		}

		if r.col.records[r.index].Active {
			// Si hay filtro, comprobamos
			if r.hasFilter {
				res := gjson.GetBytes(r.col.records[r.index].Data, r.filterPath)
				if res.Value() != r.filterExpected {
					continue // No cumple, pasamos al siguiente
				}
			}

			r.currentID = r.index
			r.currentData = r.col.records[r.index].Data
			return true
		}
		// Si no está activo (es un hueco), el bucle continúa
	}
}

// Read devuelve el ID y los datos del registro en el que estamos parados
func (r *Rows) Read() (int64, []byte) {
	return r.currentID, r.currentData
}

// Get extrae un valor del JSON actual usando su "path" sin alocar memoria.
func (r *Rows) Get(path string) gjson.Result {
	return gjson.GetBytes(r.currentData, path)
}
