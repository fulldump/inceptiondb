package records

import (
	"sync"
	"sync/atomic"
)

const recordsTurboSegmentShift = 10
const recordsTurboSegmentSize = 1 << recordsTurboSegmentShift
const recordsTurboSegmentMask = recordsTurboSegmentSize - 1

type recordsTurboValue[T any] struct {
	val T
}

type recordsTurboSegment[T any] struct {
	slots [recordsTurboSegmentSize]atomic.Pointer[recordsTurboValue[T]]
}

type recordsTurboTable[T any] struct {
	segments []*recordsTurboSegment[T]
}

type RecordsTurbo[T any] struct {
	nextID atomic.Int64
	table  atomic.Pointer[recordsTurboTable[T]]
	growMu sync.Mutex
}

func NewRecordsTurbo[T any]() *RecordsTurbo[T] {
	r := &RecordsTurbo[T]{}
	r.table.Store(&recordsTurboTable[T]{
		segments: []*recordsTurboSegment[T]{new(recordsTurboSegment[T])},
	})
	return r
}

func (r *RecordsTurbo[T]) Insert(val T) (id int64) {
	id = r.nextID.Add(1)
	segmentIndex := int(id >> recordsTurboSegmentShift)
	segment := r.ensureSegment(segmentIndex)
	offset := int(id & recordsTurboSegmentMask)
	segment.slots[offset].Store(&recordsTurboValue[T]{val: val})
	return id
}

func (r *RecordsTurbo[T]) Delete(id int64) {
	if id <= 0 {
		return
	}

	table := r.table.Load()
	segmentIndex := int(id >> recordsTurboSegmentShift)
	if segmentIndex >= len(table.segments) {
		return
	}

	segment := table.segments[segmentIndex]
	offset := int(id & recordsTurboSegmentMask)
	segment.slots[offset].Store(nil)
}

func (r *RecordsTurbo[T]) Get(id int64) (val T) {
	if id <= 0 {
		return
	}

	table := r.table.Load()
	segmentIndex := int(id >> recordsTurboSegmentShift)
	if segmentIndex >= len(table.segments) {
		return
	}

	segment := table.segments[segmentIndex]
	offset := int(id & recordsTurboSegmentMask)
	ptr := segment.slots[offset].Load()
	if ptr != nil {
		return ptr.val
	}
	return
}

func (r *RecordsTurbo[T]) ensureSegment(segmentIndex int) *recordsTurboSegment[T] {
	table := r.table.Load()
	if segmentIndex < len(table.segments) {
		return table.segments[segmentIndex]
	}

	r.growMu.Lock()
	defer r.growMu.Unlock()

	table = r.table.Load()
	if segmentIndex < len(table.segments) {
		return table.segments[segmentIndex]
	}

	newLen := len(table.segments)
	for newLen <= segmentIndex {
		newLen <<= 1
	}

	newSegments := make([]*recordsTurboSegment[T], newLen)
	copy(newSegments, table.segments)
	for i := len(table.segments); i < newLen; i++ {
		newSegments[i] = new(recordsTurboSegment[T])
	}

	r.table.Store(&recordsTurboTable[T]{segments: newSegments})
	return newSegments[segmentIndex]
}

func (r *RecordsTurbo[T]) Set(id int64, val T) {
	if id <= 0 {
		return
	}
	segmentIndex := int(id >> recordsTurboSegmentShift)
	segment := r.ensureSegment(segmentIndex)
	offset := int(id & recordsTurboSegmentMask)
	segment.slots[offset].Store(&recordsTurboValue[T]{val: val})

	for {
		curr := r.nextID.Load()
		if id <= curr || r.nextID.CompareAndSwap(curr, id) {
			break
		}
	}
}

func (r *RecordsTurbo[T]) Traverse(f func(id int64, val T) bool) {
	table := r.table.Load()
	for segIdx, seg := range table.segments {
		if seg == nil {
			continue
		}
		for offset := 0; offset < len(seg.slots); offset++ {
			ptr := seg.slots[offset].Load()
			if ptr != nil {
				id := int64(segIdx<<recordsTurboSegmentShift) | int64(offset)
				if id == 0 {
					continue
				}
				if !f(id, ptr.val) {
					return
				}
			}
		}
	}
}
