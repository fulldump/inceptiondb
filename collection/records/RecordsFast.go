package records

import (
	"sync"
)

type RecordsFast[T any] struct {
	recods         []T
	recordsLength  int64
	freeList       []int64
	freeListLength int64
	freed          []bool // freed[id] is true while id is deleted and reusable
	mutex          sync.RWMutex
}

func NewRecordsFast[T any]() *RecordsFast[T] {
	return &RecordsFast[T]{
		recods:         make([]T, 1000),
		recordsLength:  0,
		freeList:       make([]int64, 100),
		freeListLength: 0,
		mutex:          sync.RWMutex{},
	}
}

func (r *RecordsFast[T]) Insert(val T) (id int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Can reuse id; skip entries revived by Set
	for r.freeListLength > 0 {
		r.freeListLength--
		id = r.freeList[r.freeListLength]
		if !r.freed[id] {
			continue
		}
		r.freed[id] = false
		r.recods[id] = val
		return id
	}

	// New id
	id = r.recordsLength
	if id < int64(len(r.recods)) {
		r.recods[id] = val
	} else {
		r.recods = append(r.recods, val)
	}
	r.recordsLength++
	return id
}

func (r *RecordsFast[T]) Delete(id int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Check if already deleted
	if id < 0 || id >= r.recordsLength {
		return
	}
	for int64(len(r.freed)) <= id {
		r.freed = append(r.freed, false)
	}
	if r.freed[id] {
		return
	}
	r.freed[id] = true

	var zero T
	r.recods[id] = zero

	if r.freeListLength < int64(len(r.freeList)) {
		r.freeList[r.freeListLength] = id
	} else {
		r.freeList = append(r.freeList, id)
	}
	r.freeListLength++
}

func (r *RecordsFast[T]) Get(id int64) (val T) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if id < r.recordsLength {
		return r.recods[id]
	}
	return
}

func (r *RecordsFast[T]) Set(id int64, val T) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for int64(len(r.recods)) <= id {
		var zero T
		r.recods = append(r.recods, zero)
	}

	r.recods[id] = val
	if id < int64(len(r.freed)) {
		r.freed[id] = false
	}
	if id >= r.recordsLength {
		r.recordsLength = id + 1
	}
}

func (r *RecordsFast[T]) Traverse(f func(id int64, val T) bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for i := int64(0); i < r.recordsLength; i++ {
		if !f(i, r.recods[i]) {
			break
		}
	}
}
