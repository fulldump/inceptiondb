package collectionv4

import (
	"path/filepath"
	"testing"
)

func TestStoreSnappy_Basic(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "snappy_test.wal")

	rawStore, err := NewStoreCrazy(filename) // or NewStoreDisk
	if err != nil {
		t.Fatal(err)
	}

	snappyStore := NewStoreSnappy(rawStore)

	err = snappyStore.Append(OpInsert, 1, []byte("Hello Compressed World!"), true)
	if err != nil {
		t.Fatal(err)
	}
	err = snappyStore.Append(OpUpdate, 2, []byte("Another compressed message"), true)
	if err != nil {
		t.Fatal(err)
	}

	err = snappyStore.Close()
	if err != nil {
		t.Fatal(err)
	}

	rawStore2, err := NewStoreCrazy(filename)
	if err != nil {
		t.Fatal(err)
	}
	snappyStore2 := NewStoreSnappy(rawStore2)

	replayedData := make(map[int64]string)
	err = snappyStore2.Replay(func(op uint8, id int64, data []byte) error {
		replayedData[id] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if replayedData[1] != "Hello Compressed World!" {
		t.Errorf("expected 'Hello Compressed World!', got '%s'", replayedData[1])
	}
	if replayedData[2] != "Another compressed message" {
		t.Errorf("expected 'Another compressed message', got '%s'", replayedData[2])
	}

	snappyStore2.Close()
}

func BenchmarkStoreSnappy(b *testing.B) {
	dir := b.TempDir()
	filename := filepath.Join(dir, "snappy_bench.wal")

	rawStore, err := NewStoreCrazy(filename)
	if err != nil {
		b.Fatal(err)
	}
	snappyStore := NewStoreSnappy(rawStore)
	payload := []byte(`{"name":"Alice","email":"alice@example.com","age":30,"active":true,"balance":1500.50,"some_repeated_field_for_compression":"hello world hello world hello world"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = snappyStore.Append(OpInsert, int64(i), payload, false)
	}
	b.StopTimer()
	snappyStore.Close()
}

func BenchmarkStoreNoCompression(b *testing.B) {
	dir := b.TempDir()
	filename := filepath.Join(dir, "nocompress_bench.wal")

	rawStore, err := NewStoreCrazy(filename)
	if err != nil {
		b.Fatal(err)
	}
	payload := []byte(`{"name":"Alice","email":"alice@example.com","age":30,"active":true,"balance":1500.50,"some_repeated_field_for_compression":"hello world hello world hello world"}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = rawStore.Append(OpInsert, int64(i), payload, false)
	}
	b.StopTimer()
	rawStore.Close()
}
