package collectionv2

import (
	"sync"
	"sync/atomic"

	"github.com/google/btree"
)

type RowContainer interface {
	ReplaceOrInsert(row *Row)
	Delete(row *Row)
	Get(row *Row) (*Row, bool)
	Has(row *Row) bool
	Len() int
	Traverse(iterator func(i *Row) bool)
}

// --- BTree Implementation ---

type BTreeContainer struct {
	tree *btree.BTreeG[*Row]
}

func NewBTreeContainer() *BTreeContainer {
	return &BTreeContainer{
		tree: btree.NewG(32, func(a, b *Row) bool { return a.Less(b) }),
	}
}

func (b *BTreeContainer) ReplaceOrInsert(row *Row) {
	b.tree.ReplaceOrInsert(row)
}

func (b *BTreeContainer) Delete(row *Row) {
	b.tree.Delete(row)
}

func (b *BTreeContainer) Get(row *Row) (*Row, bool) {
	return b.tree.Get(row)
}

func (b *BTreeContainer) Has(row *Row) bool {
	return b.tree.Has(row)
}

func (b *BTreeContainer) Len() int {
	return b.tree.Len()
}

func (b *BTreeContainer) Traverse(iterator func(i *Row) bool) {
	b.tree.Ascend(iterator)
}

// --- SyncMap Implementation ---

type SyncMapContainer struct {
	m      sync.Map
	length int64
}

func NewSyncMapContainer() *SyncMapContainer {
	return &SyncMapContainer{}
}

func (s *SyncMapContainer) ReplaceOrInsert(row *Row) {
	_, loaded := s.m.LoadOrStore(row.I, row)
	if !loaded {
		atomic.AddInt64(&s.length, 1)
	} else {
		s.m.Store(row.I, row)
	}
}

func (s *SyncMapContainer) Delete(row *Row) {
	_, loaded := s.m.LoadAndDelete(row.I)
	if loaded {
		atomic.AddInt64(&s.length, -1)
	}
}

func (s *SyncMapContainer) Get(row *Row) (*Row, bool) {
	val, ok := s.m.Load(row.I)
	if !ok {
		return nil, false
	}
	return val.(*Row), true
}

func (s *SyncMapContainer) Has(row *Row) bool {
	_, ok := s.m.Load(row.I)
	return ok
}

func (s *SyncMapContainer) Len() int {
	return int(atomic.LoadInt64(&s.length))
}

func (s *SyncMapContainer) Traverse(iterator func(i *Row) bool) {
	s.m.Range(func(key, value any) bool {
		return iterator(value.(*Row))
	})
}

// --- Slice Implementation ---

type SliceContainer struct {
	rows []*Row
}

func NewSliceContainer() *SliceContainer {
	return &SliceContainer{
		rows: []*Row{},
	}
}

func (s *SliceContainer) ReplaceOrInsert(row *Row) {
	// Check if row already exists (by I) to update it?
	// But SliceContainer relies on I being the index.
	// If row.I is within bounds, we update?
	// Or do we always append?
	// The original collection appends and sets I.
	// But here we might receive a row that is already in the container (e.g. patch).

	if row.I >= 0 && row.I < len(s.rows) && s.rows[row.I] == row {
		// Already exists at the correct position, nothing to do?
		// Or maybe payload changed.
		return
	}

	// If it's a new row or we are forcing it in:
	// For now, let's assume append behavior for new rows.
	// But wait, ReplaceOrInsert implies "replace if exists".
	// How do we know if it exists? By pointer? By ID?
	// In BTree it uses Less.
	// In SyncMap it uses I.
	// Here, I is the index.

	// If we assume I is the index:
	if row.I >= 0 && row.I < len(s.rows) {
		s.rows[row.I] = row
		return
	}

	// Append
	row.I = len(s.rows)
	s.rows = append(s.rows, row)
}

func (s *SliceContainer) Delete(row *Row) {
	i := row.I
	if i < 0 || i >= len(s.rows) {
		return
	}
	if s.rows[i] != row {
		// Row mismatch, maybe already moved or deleted?
		return
	}

	last := len(s.rows) - 1
	s.rows[i] = s.rows[last]
	s.rows[i].I = i // Update I of the moved row
	s.rows = s.rows[:last]
}

func (s *SliceContainer) Get(row *Row) (*Row, bool) {
	if row.I < 0 || row.I >= len(s.rows) {
		return nil, false
	}
	return s.rows[row.I], true
}

func (s *SliceContainer) Has(row *Row) bool {
	if row.I < 0 || row.I >= len(s.rows) {
		return false
	}
	return s.rows[row.I] == row // Check pointer equality? Or just existence?
}

func (s *SliceContainer) Len() int {
	return len(s.rows)
}

func (s *SliceContainer) Traverse(iterator func(i *Row) bool) {
	for _, row := range s.rows {
		if !iterator(row) {
			break
		}
	}
}
