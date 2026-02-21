package collectionv4

type Rows struct {
	col   *Collection
	index int64
	// variables para el registro actual
	currentID   int64
	currentData []byte
}

func (c *Collection) Scan() *Rows {
	// Ojo: En un entorno altamente concurrente, deberías tomar un Read Lock
	// o usar un snapshot para evitar que los datos cambien mientras iteras.
	return &Rows{
		col:   c,
		index: -1,
	}
}

// Next avanza al siguiente registro válido (saltando huecos).
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
