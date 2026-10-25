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

// popFree returns a reusable local id, or 0 when there is none. Entries whose
// slot became active again through set (WAL replay) are stale and skipped.
func (s *recordsUltraShard[T]) popFree() int64 {
	for n := len(s.freeList); n > 0; n = len(s.freeList) {
		lid := s.freeList[n-1]
		s.freeList = s.freeList[:n-1]
		if !s.segments[lid>>recordsUltraSegmentShift][lid&recordsUltraSegmentMask].active {
			return lid
		}
	}
	return 0
}

func (s *recordsUltraShard[T]) insert(val T) int64 {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	lid := s.popFree()
	if lid == 0 {
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

// recordsUltraTraverseBatch is how many slots traverse copies per lock.
// Copying a batch and calling f outside the lock keeps writers (which need
// the exclusive lock) from queueing behind every reader on every slot.
const recordsUltraTraverseBatch = 128

func (s *recordsUltraShard[T]) traverse(f func(lid int64, val T) bool) bool {
	s.mutex.RLock()
	maxLid := s.localID
	s.mutex.RUnlock()

	type entry struct {
		lid int64
		val T
	}
	var batch [recordsUltraTraverseBatch]entry

	// Skip lid=0 on shard 0 because global ID 0 is reserved (means "no ID")
	for lid := int64(1); lid < maxLid; {
		n := 0
		s.mutex.RLock()
		for ; lid < maxLid && n < len(batch); lid++ {
			segIdx := lid >> recordsUltraSegmentShift
			if segIdx >= int64(len(s.segments)) {
				lid = maxLid
				break
			}
			slot := &s.segments[segIdx][lid&recordsUltraSegmentMask]
			if slot.active {
				batch[n] = entry{lid: lid, val: slot.val}
				n++
			}
		}
		s.mutex.RUnlock()

		for i := 0; i < n; i++ {
			if !f(batch[i].lid, batch[i].val) {
				return false
			}
		}
	}
	return true
}

// A picker stays on its shard for recordsUltraPickerRun inserts (locality, no
// contention between cores) and then moves recordsUltraPickerStride shards
// ahead (odd, so it visits all of them). Without moving, a single insert
// stream would put the whole collection in one shard and serialize every
// later read and write on its lock.
const recordsUltraPickerRun = 64
const recordsUltraPickerStride = 211

type recordsUltraPicker struct {
	idx int
	n   int
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
		return &recordsUltraPicker{idx: idx}
	}

	return r
}

func (r *RecordsUltra[T]) Insert(val T) int64 {
	p := r.picker.Get().(*recordsUltraPicker)
	id := r.shards[p.idx].insert(val)
	p.n++
	if p.n == recordsUltraPickerRun {
		p.n = 0
		p.idx = (p.idx + recordsUltraPickerStride) & recordsUltraShardMask
	}
	r.picker.Put(p)
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

func (r *RecordsUltra[T]) Traverse(f func(id int64, val T) bool) {
	for i := 0; i < recordsUltraNumShards; i++ {
		shard := r.shards[i]
		cont := shard.traverse(func(lid int64, val T) bool {
			id := (lid << recordsUltraShardBits) | int64(shard.shardIndex)
			return f(id, val)
		})
		if !cont {
			break
		}
	}
}

func (r *RecordsUltra[T]) MaxID() int64 {
	var maxID int64
	for i := 0; i < recordsUltraNumShards; i++ {
		shard := r.shards[i]
		shard.mutex.RLock()
		localID := shard.localID
		shard.mutex.RUnlock()
		id := (localID << recordsUltraShardBits) | int64(shard.shardIndex)
		if id > maxID {
			maxID = id
		}
	}
	return maxID
}
