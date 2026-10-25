package records

import (
	"sync"
)

type RecordsCorrect[T any] struct {
	vals   map[int64]T
	mutex  sync.RWMutex
	lastid int64
}

func NewRecordsCorrect[T any]() *RecordsCorrect[T] {
	return &RecordsCorrect[T]{
		vals:   make(map[int64]T),
		mutex:  sync.RWMutex{},
		lastid: -1,
	}
}

func (r *RecordsCorrect[T]) Insert(val T) (id int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.lastid++
	id = r.lastid
	r.vals[id] = val
	return id
}

func (r *RecordsCorrect[T]) Delete(id int64) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	delete(r.vals, id)
}

func (r *RecordsCorrect[T]) Get(id int64) (val T) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.vals[id]
}

func (r *RecordsCorrect[T]) Set(id int64, val T) {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	r.vals[id] = val
	if id > r.lastid {
		r.lastid = id
	}
}

func (r *RecordsCorrect[T]) Traverse(f func(id int64, val T) bool) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	for id, val := range r.vals {
		if !f(id, val) {
			break
		}
	}
}
