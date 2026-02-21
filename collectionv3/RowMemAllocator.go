package collectionv3

type MemAllocator[T any] struct {
	rows []slot[T]
	free []int // free-list (stack)
}

type slot[T any] struct {
	state slotState
	row   T
}

type slotState uint8

const (
	slotEmpty slotState = iota
	slotAlive
)

func NewMemAllocator[T any]() *MemAllocator[T] {
	return &MemAllocator[T]{
		rows: make([]slot[T], 0),
		free: make([]int, 0),
	}
}

func (a *MemAllocator[T]) Alloc(row T) int {
	var idx int

	if n := len(a.free); n > 0 {
		// reuse slot
		idx = a.free[n-1]
		a.free = a.free[:n-1]
	} else {
		// grow array
		idx = len(a.rows)
		a.rows = append(a.rows, slot[T]{})
	}

	a.rows[idx].row = row
	a.rows[idx].state = slotAlive
	return idx
}

func (a *MemAllocator[T]) Free(id int) {
	if id < 0 || id >= len(a.rows) {
		return
	}

	if a.rows[id].state != slotAlive {
		return
	}

	var zero T
	a.rows[id].row = zero
	a.rows[id].state = slotEmpty
	a.free = append(a.free, id)
}

func (a *MemAllocator[T]) Get(id int) (T, bool) {
	if id < 0 || id >= len(a.rows) {
		var zero T
		return zero, false
	}

	sl := a.rows[id]
	if sl.state != slotAlive {
		var zero T
		return zero, false
	}

	return sl.row, true
}
