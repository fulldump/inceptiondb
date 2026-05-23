package collection

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

// FastUUID generates UUID v4 strings using batched crypto/rand reads.
// Instead of calling crypto/rand.Read(16) per UUID (= 1 syscall per UUID),
// it reads 4KB at once (256 UUIDs worth) and serves them from a pool buffer.
// With sync.Pool, each goroutine gets its own buffer, eliminating contention.

const uuidBatchSize = 256
const uuidBytes = 16

type uuidBatch struct {
	buf [uuidBatchSize * uuidBytes]byte
	pos int
}

var uuidPool = sync.Pool{
	New: func() any {
		b := &uuidBatch{}
		rand.Read(b.buf[:])
		return b
	},
}

func FastUUID() string {
	b := uuidPool.Get().(*uuidBatch)

	if b.pos >= len(b.buf) {
		rand.Read(b.buf[:])
		b.pos = 0
	}

	var raw [uuidBytes]byte
	copy(raw[:], b.buf[b.pos:b.pos+uuidBytes])
	b.pos += uuidBytes

	uuidPool.Put(b)

	// Set UUID version 4 and variant RFC 4122
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	// Format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
	var uuid [36]byte
	hex.Encode(uuid[0:8], raw[0:4])
	uuid[8] = '-'
	hex.Encode(uuid[9:13], raw[4:6])
	uuid[13] = '-'
	hex.Encode(uuid[14:18], raw[6:8])
	uuid[18] = '-'
	hex.Encode(uuid[19:23], raw[8:10])
	uuid[23] = '-'
	hex.Encode(uuid[24:36], raw[10:16])

	return string(uuid[:])
}
