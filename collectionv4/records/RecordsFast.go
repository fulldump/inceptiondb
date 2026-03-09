package records

import (
	"sync"
)

type RecordsFast[T any] struct {
	recods         []T
	recordsLength  int64
	freeList       []int64
	freeListLength int64
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

	// Can reuse id
	if r.freeListLength > 0 {
		r.freeListLength--
		id = r.freeList[r.freeListLength]
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
	if id >= r.recordsLength {
		return
	}

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
	if id >= r.recordsLength {
		r.recordsLength = id + 1
	}
}
