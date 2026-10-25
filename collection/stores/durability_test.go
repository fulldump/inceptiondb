package stores

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestStoreAsyncCloseWhileAppending(t *testing.T) {
	for round := 0; round < 20; round++ {
		filename := filepath.Join(t.TempDir(), "wal")
		disk, err := NewStoreDisk(filename)
		if err != nil {
			t.Fatal(err)
		}
		store := NewStoreAsync(disk)

		var mu sync.Mutex
		accepted := map[int64]bool{}
		wg := sync.WaitGroup{}
		for w := 0; w < 8; w++ {
			wg.Add(1)
			go func(w int) {
				defer wg.Done()
				for i := 0; i < 200; i++ {
					id := int64(w*1000 + i + 1)
					if err := store.Append(OpInsert, id, []byte("x"), i%10 == 0); err != nil {
						if !errors.Is(err, ErrStoreClosed) {
							t.Error(err)
						}
						return
					}
					mu.Lock()
					accepted[id] = true
					mu.Unlock()
				}
			}(w)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
		wg.Wait()

		// Every accepted append must be in the WAL
		disk, _ = NewStoreDisk(filename)
		replayed := map[int64]bool{}
		if err := disk.Replay(func(op uint8, id int64, data []byte) error {
			replayed[id] = true
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		disk.Close()
		for id := range accepted {
			if !replayed[id] {
				t.Fatalf("round %d: accepted append %d not in WAL", round, id)
			}
		}
	}
}

func TestStoreAsyncSyncWaitsForQueuedAppends(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "wal")
	disk, _ := NewStoreDisk(filename)
	store := NewStoreAsync(disk)
	defer store.Close()

	for i := 1; i <= 10000; i++ {
		store.Append(OpInsert, int64(i), []byte(`{}`), false)
	}
	if err := store.Sync(); err != nil {
		t.Fatal(err)
	}

	// Without closing (crash), the file must already hold every record
	reader, _ := NewStoreDisk(filename)
	defer reader.Close()
	n := 0
	reader.Replay(func(op uint8, id int64, data []byte) error {
		n++
		return nil
	})
	if n != 10000 {
		t.Fatalf("records on disk after Sync: want 10000, got %d", n)
	}
}

func TestStoreAsyncErrorIsSticky(t *testing.T) {
	want := errors.New("disk full")
	store := NewStoreAsync(&failingStore{appendErr: want})
	defer store.Close()

	store.Append(OpInsert, 1, []byte(`{}`), false)
	if err := store.Sync(); !errors.Is(err, want) {
		t.Fatalf("Sync error = %v, want %v", err, want)
	}
	if err := store.Append(OpInsert, 2, []byte(`{}`), false); !errors.Is(err, want) {
		t.Fatalf("Append after failure = %v, want %v", err, want)
	}
}

func TestTornTailAllBackends(t *testing.T) {
	backends := map[string]func(string) (Store, error){
		"disk":  func(f string) (Store, error) { return NewStoreDisk(f) },
		"crazy": func(f string) (Store, error) { return NewStoreCrazy(f) },
		"json":  func(f string) (Store, error) { return NewStoreJson(f) },
	}
	for name, open := range backends {
		t.Run(name, func(t *testing.T) {
			filename := filepath.Join(t.TempDir(), "wal")
			store, _ := open(filename)
			for i := 1; i <= 5; i++ {
				store.Append(OpInsert, int64(i), []byte(fmt.Sprintf(`{"i":%d}`, i)), false)
			}
			store.Close()

			info, _ := os.Stat(filename)
			os.Truncate(filename, info.Size()-2)

			count := func() int {
				store, _ := open(filename)
				defer store.Close()
				n := 0
				if err := store.Replay(func(op uint8, id int64, data []byte) error {
					n++
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				return n
			}
			if n := count(); n != 4 {
				t.Fatalf("records after torn tail: want 4, got %d", n)
			}
			// Truncated: the next append starts at a record boundary
			store, _ = open(filename)
			store.Append(OpInsert, 6, []byte(`{"i":6}`), true)
			store.Close()
			if n := count(); n != 5 {
				t.Fatalf("records after append: want 5, got %d", n)
			}
		})
	}
}

func TestZeroFilledTailIsTruncated(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "wal")
	store, _ := NewStoreDisk(filename)
	store.Append(OpInsert, 1, []byte(`{}`), true)
	store.Close()

	f, _ := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0666)
	f.Write(make([]byte, 4096))
	f.Close()

	store, _ = NewStoreDisk(filename)
	defer store.Close()
	n := 0
	if err := store.Replay(func(op uint8, id int64, data []byte) error {
		n++
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("records: want 1, got %d", n)
	}
}
