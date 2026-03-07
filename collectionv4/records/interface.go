package records

type Records[T any] interface {
	Insert(val T) (id int64)
	Delete(id int64)
	Get(id int64) (val T)
}
