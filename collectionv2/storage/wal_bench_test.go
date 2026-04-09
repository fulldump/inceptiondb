package storage

import (
	"crypto/rand"
	"os"
	"testing"
	"time"
)

func BenchmarkJSON(b *testing.B) {
	os.Remove("/tmp/test.json")
	s, _ := NewJSONStorage("/tmp/test.json")
	defer s.Close()
	defer os.Remove("/tmp/test.json")

	payload := make([]byte, 100)
	rand.Read(payload)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.PersistInsert(1, time.Now().UnixNano(), payload)
		}
	})
}

func BenchmarkWAL(b *testing.B) {
	os.Remove("/tmp/test.wal")
	s, _ := NewWALStorage("/tmp/test.wal")
	defer s.Close()
	defer os.Remove("/tmp/test.wal")

	payload := make([]byte, 100)
	rand.Read(payload)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			s.PersistInsert(1, time.Now().UnixNano(), payload)
		}
	})
}
