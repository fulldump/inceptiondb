package collectionv4

// Rows representa el iterador de resultados
type Rows interface {
	Next() bool
	Read() (int64, []byte)
	Close()
}

// Store define el contrato interno de almacenamiento hiper-rápido de registros crudos
type Store interface {
	// Reserva un ID nuevo o de la freelist (para poder escribir el WAL antes de comitear a la RAM)
	AllocID() int64
	// Libera un ID reservado si falló la operación
	RollbackID(id int64)

	// Fused Insert for faster operations without closures
	Insert(data []byte, journal *Journal) (int64, error)

	// Almacena o pisa un registro
	Set(id int64, data []byte)
	// Elimina un registro de la memoria
	Delete(id int64)
	// Recupera un registro
	Get(id int64) ([]byte, bool)

	// Inicia un iterador sobre todos los registros
	Scan() Rows

	// Limpia las estructuras (usado en Recover)
	Reset()

	// Reconstruye la capa de ids borrados
	RebuildFreeList()

	// Obtiene la longitud para debug
	GetLen() int
	// Obtiene cuantos huecos hay
	GetFreeListLen() int
}
