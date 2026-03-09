package records

import (
	"sync"
	"sync/atomic"
)

const recordsUltraShardBits = 9
const recordsUltraNumShards = 1 << recordsUltraShardBits // 512
const recordsUltraShardMask = recordsUltraNumShards - 1

const recordsUltraSegmentShift = 12
const recordsUltraSegmentSize = 1 << recordsUltraSegmentShift // 4096
const recordsUltraSegmentMask = recordsUltraSegmentSize - 1

type ultraSlot[T any] struct {
	val    T
	active bool
}

type recordsUltraShard[T any] struct {
	_          [64]byte // padding prevents false sharing between shards
	mutex      sync.RWMutex
	shardIndex int
	localID    int64
	freeList   []int64
	segments   [][]ultraSlot[T]
	_          [64]byte // padding
}

func newRecordsUltraShard[T any](shardIndex int) *recordsUltraShard[T] {
	return &recordsUltraShard[T]{
		shardIndex: shardIndex,
		localID:    1, // start from 1 to guarantee id > 0
		segments:   [][]ultraSlot[T]{make([]ultraSlot[T], recordsUltraSegmentSize)},
	}
}

func (s *recordsUltraShard[T]) insert(val T) int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	var lid int64
	if n := len(s.freeList); n > 0 {
		lid = s.freeList[n-1]
		s.freeList = s.freeList[:n-1]
	} else {
		lid = s.localID
		s.localID++

		segIdx := lid >> recordsUltraSegmentShift
		if segIdx >= int64(len(s.segments)) {
			s.segments = append(s.segments, make([]ultraSlot[T], recordsUltraSegmentSize))
		}
	}

	segIdx := lid >> recordsUltraSegmentShift
	slot := &s.segments[segIdx][lid&recordsUltraSegmentMask]
	slot.val = val
	slot.active = true

	return (lid << recordsUltraShardBits) | int64(s.shardIndex)
}

func (s *recordsUltraShard[T]) get(lid int64) T {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	segIdx := lid >> recordsUltraSegmentShift
	if segIdx < int64(len(s.segments)) {
		slot := &s.segments[segIdx][lid&recordsUltraSegmentMask]
		if slot.active {
			return slot.val
		}
	}
	var zero T
	return zero
}

func (s *recordsUltraShard[T]) delete(lid int64) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	segIdx := lid >> recordsUltraSegmentShift
	if segIdx < int64(len(s.segments)) {
		slot := &s.segments[segIdx][lid&recordsUltraSegmentMask]
		if slot.active {
			slot.active = false
			var zero T
			slot.val = zero
			s.freeList = append(s.freeList, lid)
		}
	}
}

func (s *recordsUltraShard[T]) set(lid int64, val T) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	segIdx := lid >> recordsUltraSegmentShift
	for segIdx >= int64(len(s.segments)) {
		s.segments = append(s.segments, make([]ultraSlot[T], recordsUltraSegmentSize))
	}

	slot := &s.segments[segIdx][lid&recordsUltraSegmentMask]
	slot.val = val
	slot.active = true

	if lid >= s.localID {
		s.localID = lid + 1
	}
}

type RecordsUltra[T any] struct {
	shards [recordsUltraNumShards]*recordsUltraShard[T]
	picker sync.Pool
}

func NewRecordsUltra[T any]() *RecordsUltra[T] {
	r := &RecordsUltra[T]{}
	for i := 0; i < recordsUltraNumShards; i++ {
		r.shards[i] = newRecordsUltraShard[T](i)
	}

	var pickerCounter atomic.Uint32
	r.picker.New = func() any {
		idx := int(pickerCounter.Add(1)-1) & recordsUltraShardMask
		return &idx
	}

	return r
}

func (r *RecordsUltra[T]) Insert(val T) int64 {
	idxPtr := r.picker.Get().(*int)
	idx := *idxPtr
	id := r.shards[idx].insert(val)
	r.picker.Put(idxPtr)
	return id
}

func (r *RecordsUltra[T]) Get(id int64) T {
	if id <= 0 {
		var zero T
		return zero
	}
	shardIndex := int(id & recordsUltraShardMask)
	localID := id >> recordsUltraShardBits
	return r.shards[shardIndex].get(localID)
}

func (r *RecordsUltra[T]) Delete(id int64) {
	if id <= 0 {
		return
	}
	shardIndex := int(id & recordsUltraShardMask)
	localID := id >> recordsUltraShardBits
	r.shards[shardIndex].delete(localID)
}

func (r *RecordsUltra[T]) Set(id int64, val T) {
	if id <= 0 {
		return
	}
	shardIndex := int(id & recordsUltraShardMask)
	localID := id >> recordsUltraShardBits
	r.shards[shardIndex].set(localID, val)
}
