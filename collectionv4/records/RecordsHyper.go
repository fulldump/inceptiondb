package records

import (
	"sync"
	"sync/atomic"
)

const recordsHyperShardBits = 11
const recordsHyperNumShards = 1 << recordsHyperShardBits // 2048
const recordsHyperShardMask = recordsHyperNumShards - 1

const recordsHyperSegmentShift = 12
const recordsHyperSegmentSize = 1 << recordsHyperSegmentShift // 4096
const recordsHyperSegmentMask = recordsHyperSegmentSize - 1

type hyperSlot[T any] struct {
	val    T
	active bool
}

type recordsHyperShard[T any] struct {
	_          [64]byte // padding prevents false sharing
	mutex      sync.Mutex
	shardIndex int
	localID    int64
	freeList   []int64
	segments   [][]hyperSlot[T]
	_          [64]byte // padding
}

func newRecordsHyperShard[T any](shardIndex int) *recordsHyperShard[T] {
	return &recordsHyperShard[T]{
		shardIndex: shardIndex,
		localID:    1, // 0 is invalid ID
		segments:   [][]hyperSlot[T]{make([]hyperSlot[T], recordsHyperSegmentSize)},
	}
}

func (s *recordsHyperShard[T]) insert(val T) int64 {
	s.mutex.Lock()

	var lid int64
	var freeLen = len(s.freeList)
	if freeLen > 0 {
		lid = s.freeList[freeLen-1]
		s.freeList = s.freeList[:freeLen-1]
	} else {
		lid = s.localID
		s.localID++

		segIdx := lid >> recordsHyperSegmentShift
		if segIdx >= int64(len(s.segments)) {
			s.segments = append(s.segments, make([]hyperSlot[T], recordsHyperSegmentSize))
		}
	}

	segIdx := lid >> recordsHyperSegmentShift
	slot := &s.segments[segIdx][lid&recordsHyperSegmentMask]
	slot.val = val
	slot.active = true

	s.mutex.Unlock()

	return (lid << recordsHyperShardBits) | int64(s.shardIndex)
}

func (s *recordsHyperShard[T]) get(lid int64) T {
	s.mutex.Lock()
	
	segIdx := lid >> recordsHyperSegmentShift
	if segIdx < int64(len(s.segments)) {
		slot := &s.segments[segIdx][lid&recordsHyperSegmentMask]
		if slot.active {
			val := slot.val
			s.mutex.Unlock()
			return val
		}
	}
	
	s.mutex.Unlock()
	var zero T
	return zero
}

func (s *recordsHyperShard[T]) delete(lid int64) {
	s.mutex.Lock()

	segIdx := lid >> recordsHyperSegmentShift
	if segIdx < int64(len(s.segments)) {
		slot := &s.segments[segIdx][lid&recordsHyperSegmentMask]
		if slot.active {
			slot.active = false
			var zero T
			slot.val = zero
			s.freeList = append(s.freeList, lid)
		}
	}
	
	s.mutex.Unlock()
}

type RecordsHyper[T any] struct {
	shards [recordsHyperNumShards]*recordsHyperShard[T]
	picker sync.Pool
}

func NewRecordsHyper[T any]() *RecordsHyper[T] {
	r := &RecordsHyper[T]{}
	for i := 0; i < recordsHyperNumShards; i++ {
		r.shards[i] = newRecordsHyperShard[T](i)
	}

	var pickerCounter atomic.Uint32
	r.picker.New = func() any {
		idx := int(pickerCounter.Add(1)-1) & recordsHyperShardMask
		return &idx
	}

	return r
}

func (r *RecordsHyper[T]) Insert(val T) int64 {
	idxPtr := r.picker.Get().(*int)
	idx := *idxPtr
	id := r.shards[idx].insert(val)
	r.picker.Put(idxPtr)
	return id
}

func (r *RecordsHyper[T]) Get(id int64) T {
	if id <= 0 {
		var zero T
		return zero
	}
	shardIndex := int(id & recordsHyperShardMask)
	localID := id >> recordsHyperShardBits
	return r.shards[shardIndex].get(localID)
}

func (r *RecordsHyper[T]) Delete(id int64) {
	if id <= 0 {
		return
	}
	shardIndex := int(id & recordsHyperShardMask)
	localID := id >> recordsHyperShardBits
	r.shards[shardIndex].delete(localID)
}
