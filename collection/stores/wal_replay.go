package stores

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"runtime"
)

const maxOp = OpSetDefaults

// walFormat describes the binary record layout shared by StoreDisk and StoreCrazy:
// OpCode(1) + ID(8) + Length(4) [+ CRC32(4)] + payload.
type walFormat struct {
	headerSize int
	crc        bool
}

// replayWAL reads every record of the file at path and calls fn.
//
// A record cut at the end of the file (crash or power loss in the middle of a
// write) is not an error: it was never acknowledged as durable, so it is
// dropped and the file is truncated to the last complete record. Invalid data
// followed by more data is reported as corruption instead, so valid records
// are never discarded silently.
func replayWAL(f *os.File, format walFormat, fn func(op uint8, id int64, data []byte) error) error {
	info, err := f.Stat()
	if err != nil {
		return err
	}
	size := info.Size()

	reader, err := os.Open(f.Name())
	if err != nil {
		return err
	}
	defer reader.Close()

	r := bufio.NewReaderSize(reader, 4*1024*1024)
	header := make([]byte, format.headerSize)
	var offset int64

	// torn reports whether the invalid record at offset is the tail of the
	// log: either it does not fit in the file or everything after it is zero
	// (preallocated blocks after a power loss).
	torn := func(recordEnd int64) (bool, error) {
		if recordEnd >= size {
			return true, nil
		}
		return zeroFrom(reader, offset)
	}

	for offset < size {
		if size-offset < int64(format.headerSize) {
			return truncateTail(f, offset, size, "header cut")
		}
		if _, err := io.ReadFull(r, header); err != nil {
			return err
		}

		op := header[0]
		id := int64(binary.LittleEndian.Uint64(header[1:9]))
		length := int64(binary.LittleEndian.Uint32(header[9:13]))
		recordEnd := offset + int64(format.headerSize) + length

		if op == 0 || op > maxOp || recordEnd > size {
			isTail, err := torn(recordEnd)
			if err != nil {
				return err
			}
			if isTail {
				return truncateTail(f, offset, size, "record cut")
			}
			return fmt.Errorf("WAL corrupted at offset %d: invalid record header (op %d, length %d)", offset, op, length)
		}

		var data []byte
		if length > 0 {
			data = make([]byte, length)
			if _, err := io.ReadFull(r, data); err != nil {
				return err
			}
		}

		if format.crc {
			expected := binary.LittleEndian.Uint32(header[13:17])
			if actual := crc32.Checksum(data, crcTable); actual != expected {
				isTail, err := torn(recordEnd)
				if err != nil {
					return err
				}
				if isTail {
					return truncateTail(f, offset, size, "checksum mismatch in last record")
				}
				return fmt.Errorf("WAL corrupted at offset %d (id %d): CRC expected %x, got %x", offset, id, expected, actual)
			}
		}

		if err := fn(op, id, data); err != nil {
			return err
		}
		offset = recordEnd
	}

	return nil
}

// zeroFrom reports whether every byte from offset to the end of the file is zero.
func zeroFrom(f *os.File, offset int64) (bool, error) {
	r := bufio.NewReader(io.NewSectionReader(f, offset, 1<<62))
	for {
		b, err := r.ReadByte()
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		if b != 0 {
			return false, nil
		}
	}
}

func truncateTail(f *os.File, offset, size int64, reason string) error {
	fmt.Printf("WARNING: %s: %s at offset %d, discarding last %d bytes\n", f.Name(), reason, offset, size-offset)
	if err := f.Truncate(offset); err != nil {
		return fmt.Errorf("truncate torn WAL tail: %w", err)
	}
	return f.Sync()
}

// openWAL opens (or creates) an append-only WAL file. When the file is new,
// its directory is fsynced so the file itself survives a power loss.
func openWAL(path string) (*os.File, error) {
	_, statErr := os.Stat(path)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	if os.IsNotExist(statErr) {
		if err := SyncDir(filepath.Dir(path)); err != nil {
			f.Close()
			return nil, err
		}
	}
	return f, nil
}

// SyncDir fsyncs a directory so that entries created or renamed in it are durable.
func SyncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil // directories cannot be fsynced on Windows
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}
