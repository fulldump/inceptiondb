package collectionv3

type RowAllocator[T any] interface {
	Alloc(row T) int
	Free(id int)
	Get(id int) (T, bool)
}
