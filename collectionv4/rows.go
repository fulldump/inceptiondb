package collectionv4

type Rows struct {
	col   *Collection
	index int64
	// variables para el registro actual
	currentID   int64
	currentData []byte
}

func (c *Collection) Scan() *Rows {
	return &Rows{
		col:   c,
		index: 0, // Starts at 0 (IDs start at 1 if Ultra is used, or from DB)
	}
}

// Next avanza al siguiente registro válido (saltando huecos).
// Devuelve false cuando no hay más registros.
func (r *Rows) Next() bool {
	maxID := r.col.maxID.Load()

	for {
		if r.index > maxID {
			return false // Fin de la tabla
		}

		rec := r.col.records.Get(r.index)
		id := r.index
		r.index++ // Avanzamos el índice para la próxima iteración

		if rec.Active {
			r.currentID = id
			r.currentData = rec.Data
			return true
		}
		// Si no está activo (es un hueco o no existe), el bucle continúa
	}
}

// Read devuelve el ID y los datos del registro en el que estamos parados
func (r *Rows) Read() (int64, []byte) {
	return r.currentID, r.currentData
}
