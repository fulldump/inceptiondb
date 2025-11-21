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
