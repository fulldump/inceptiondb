package collectionv4

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"sync"
)

const (
	OpInsert uint8 = 1
	OpDelete uint8 = 2
	OpUpdate uint8 = 3
)

var crcTable = crc32.MakeTable(crc32.Castagnoli)

type Store struct {
	file   *os.File
	writer *bufio.Writer
	mu     sync.Mutex
}

func NewStore(path string) (*Store, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Store{
		file:   f,
		writer: bufio.NewWriterSize(f, 1024*1024), // Buffer de 1MB para no castigar el disco
	}, nil
}

// Append escribe la operación en el WAL.
// Header (17 bytes) = OpCode(1) + ID(8) + Length(4) + CRC32(4)
func (s *Store) Append(op uint8, id int64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	header := make([]byte, 17)
	header[0] = op
	binary.LittleEndian.PutUint64(header[1:], uint64(id))

	length := uint32(len(data))
	binary.LittleEndian.PutUint32(header[9:], length)

	checksum := crc32.Checksum(data, crcTable)
	binary.LittleEndian.PutUint32(header[13:], checksum)

	// Escribir header y luego el payload (Zero-copy del payload)
	if _, err := s.writer.Write(header); err != nil {
		return err
	}
	if length > 0 {
		if _, err := s.writer.Write(data); err != nil {
			return err
		}
	}

	// Nota: Podrías llamar a s.writer.Flush() aquí o dejarlo para un worker asíncrono
	return nil
}

// Flush vacía el buffer de Go hacia el Sistema Operativo
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.writer.Flush()
}

// Sync asegura que los datos pasen del Sistema Operativo al disco físico (fsync)
func (s *Store) Sync() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. Primero vaciamos el buffer de Go
	if err := s.writer.Flush(); err != nil {
		return err
	}
	// 2. Obligamos al disco duro a escribir físicamente
	return s.file.Sync()
}

// Close cierra el Journal de forma segura
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Vaciamos todo antes de cerrar
	if err := s.writer.Flush(); err != nil {
		return err
	}
	if err := s.file.Sync(); err != nil {
		return err
	}
	return s.file.Close()
}

// Replay lee el WAL de principio a fin y ejecuta la función fn por cada registro.
// Si encuentra un registro corrupto, se detiene y avisa.
func (s *Store) Replay(fn func(op uint8, id int64, data []byte) error) error {
	// Abrimos el archivo en modo solo lectura para la recuperación
	f, err := os.Open(s.file.Name())
	if err != nil {
		return err
	}
	defer f.Close()

	reader := bufio.NewReaderSize(f, 1024*1024) // Buffer de 1MB para lectura rápida
	header := make([]byte, 17)

	for {
		// 1. Leer el Header (17 bytes)
		_, err := io.ReadFull(reader, header)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break // Fin del archivo, recuperación terminada con éxito
			}
			if errors.Is(err, io.ErrUnexpectedEOF) {
				// El archivo se cortó a la mitad de un header (ej. corte de luz)
				return fmt.Errorf("WAL header cortado de forma abrupta")
			}
			return err
		}

		op := header[0]
		id := int64(binary.LittleEndian.Uint64(header[1:9]))
		length := binary.LittleEndian.Uint32(header[9:13])
		expectedCRC := binary.LittleEndian.Uint32(header[13:17])

		// 2. Leer el Payload (el JSON)
		var data []byte
		if length > 0 {
			data = make([]byte, length)
			_, err = io.ReadFull(reader, data)
			if err != nil {
				return fmt.Errorf("WAL payload cortado en ID %d: %v", id, err)
			}

			// 3. Validar la integridad con CRC32
			actualCRC := crc32.Checksum(data, crcTable)
			if actualCRC != expectedCRC {
				// ¡Peligro! Corrupción detectada
				return fmt.Errorf("corrupción de datos en ID %d: CRC esperado %x, obtenido %x", id, expectedCRC, actualCRC)
			}
		}

		// 4. Procesar el registro en la memoria
		if err := fn(op, id, data); err != nil {
			return err
		}
	}

	return nil
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
