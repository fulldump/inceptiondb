package stores

import (
	"errors"
	"testing"
)

type failingStore struct {
	appendErr error
}

func (s *failingStore) Append(op uint8, id int64, data []byte, sync bool) error {
	return s.appendErr
}

func (s *failingStore) Flush() error { return nil }

func (s *failingStore) Sync() error { return nil }

func (s *failingStore) Close() error { return nil }

func (s *failingStore) Replay(fn func(op uint8, id int64, data []byte) error) error { return nil }

func TestStoreAsyncReportsAppendError(t *testing.T) {
	want := errors.New("append failed")
	store := NewStoreAsync(&failingStore{appendErr: want})
	defer store.Close()

	if err := store.Append(OpInsert, 1, []byte(`{"id":1}`), true); !errors.Is(err, want) {
		t.Fatalf("Append error = %v, want %v", err, want)
	}
}
