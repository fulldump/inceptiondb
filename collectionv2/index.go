package collectionv2

type Index interface {
	AddRow(row *Row) error
	RemoveRow(row *Row) error
	Traverse(options []byte, f func(row *Row) bool) // todo: return error?
	GetType() string
	GetOptions() interface{}
}
