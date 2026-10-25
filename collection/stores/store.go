package stores

import (
	"bufio"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"os"
	"sync"
	"sync/atomic"
)

const (
	OpInsert      uint8 = 1
	OpDelete      uint8 = 2
	OpUpdate      uint8 = 3
	OpCreateIndex uint8 = 4
	OpDropIndex   uint8 = 5
	OpSetDefaults uint8 = 6
)

var storeDiskBufferPool = &sync.Pool{
	New: func() interface{} {
		// preallocate 1KB buffer minimum
		b := make([]byte, 0, 1024)
		return &b
	},
}

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type Store interface {
	Append(op uint8, id int64, data []byte, sync bool) error
	Flush() error
	Sync() error
	Close() error
	Replay(fn func(op uint8, id int64, data []byte) error) error
}

type StoreDisk struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
	closed atomic.Bool
	failed error // sticky: set when a write or fsync fails, guarded by mu
}

func NewStoreDisk(path string) (*StoreDisk, error) {
	f, err := openWAL(path)
	if err != nil {
		return nil, err
	}
	s := &StoreDisk{
		file:   f,
		writer: bufio.NewWriterSize(f, 1024*1024), // Buffer de 1MB para no castigar el disco
	}

	return s, nil
}

// Append escribe la operación en el WAL.
// Header (17 bytes) = OpCode(1) + ID(8) + Length(4) + CRC32(4)
func (s *StoreDisk) Append(op uint8, id int64, data []byte, sync bool) error {
	if s.closed.Load() {
		return errors.New("StoreDisk closed")
	}

	var header [17]byte
	header[0] = op
	binary.LittleEndian.PutUint64(header[1:9], uint64(id))

	length := uint32(len(data))
	binary.LittleEndian.PutUint32(header[9:13], length)

	checksum := crc32.Checksum(data, crcTable)
	binary.LittleEndian.PutUint32(header[13:17], checksum)

	bufPtr := storeDiskBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	buf = append(buf, header[:]...)
	buf = append(buf, data...)

	// Escribir header y luego el payload de una (Single Write)
	s.mu.Lock()
	err := s.failed
	if err == nil {
		if _, err = s.writer.Write(buf); err != nil {
			s.failed = err
		} else if sync {
			err = s.syncLocked()
		}
	}
	s.mu.Unlock()

	*bufPtr = buf
	storeDiskBufferPool.Put(bufPtr)

	return err
}

// Flush vacía el buffer de Go hacia el Sistema Operativo
func (s *StoreDisk) Flush() error {
	if s.closed.Load() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.failed != nil {
		return s.failed
	}
	if err := s.writer.Flush(); err != nil {
		s.failed = err
		return err
	}
	return nil
}

// Sync asegura que los datos pasen del Sistema Operativo al disco físico (fsync)
func (s *StoreDisk) Sync() error {
	if s.closed.Load() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.syncLocked()
}

// syncLocked flushes the buffer and fsyncs the file. After a failure the
// store refuses every write: the kernel may have dropped the dirty pages, so
// a later successful fsync would not mean the data is on disk.
func (s *StoreDisk) syncLocked() error {
	if s.failed != nil {
		return s.failed
	}
	if err := s.writer.Flush(); err != nil {
		s.failed = err
		return err
	}
	if err := s.file.Sync(); err != nil {
		s.failed = err
		return err
	}
	return nil
}

// Close cierra el Journal de forma segura
func (s *StoreDisk) Close() error {
	if s.closed.Swap(true) {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err := s.syncLocked()
	if closeErr := s.file.Close(); err == nil {
		err = closeErr
	}
	return err
}

// Replay lee el WAL de principio a fin y ejecuta la función fn por cada registro.
// Si encuentra un registro corrupto, se detiene y avisa.
func (s *StoreDisk) Replay(fn func(op uint8, id int64, data []byte) error) error {
	return replayWAL(s.file, walFormat{headerSize: 17, crc: true}, fn)
}

/*
importante:
writer.Flush(): Mueve los bytes de tu programa en Go a la memoria del Sistema
Operativo (OS cache). Es rápido. Si tu programa crashea pero el PC sigue encendido,
los datos están a salvo (el OS los escribirá). Pero si hay un corte de luz, se pierden.

file.Sync() (fsync): Le dice al disco duro: "No me devuelvas el control hasta que
los electrones estén grabados en la placa magnética/chip flash". Es lento (puede
tardar milisegundos), pero a prueba de apagones.

*/

/*
Estrategias:
Estrategia A: Máxima Seguridad (Pobre Rendimiento)
Llamas a Sync() dentro del metodo Append en cada Insert o Delete. Así funcionan
SQLite o PostgreSQL por defecto. Es 100% seguro (ACID), pero tu base de datos
estará limitada a los IOPS de tu disco duro (quizás unas pocos miles de escrituras
por segundo).

Estrategia B: Equilibrada (Bufio nativo)
Dejas que el bufio.Writer se llene solo (cuando llega a 1MB hace auto-flush) y
llamas a Close() solo al apagar el servidor.

Peligro: Si se corta la luz, pierdes hasta 1MB de datos recientes.

Estrategia C: "Background Flusher" (El estándar In-Memory moderno)
Esta es la que usan sistemas como Redis (con su AOF) o BadgerDB. Creas una
Goroutine que hace Sync cada segundo en segundo plano. Así las escrituras en RAM
son instantáneas, pero la ventana de pérdida de datos ante un corte de luz fatal
es de solo 1 segundo.
*/
