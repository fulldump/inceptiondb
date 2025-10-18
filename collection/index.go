package collection

type Index interface {
	AddRow(row *Row, item map[string]any) error
	RemoveRow(row *Row, item map[string]any) error
	Traverse(options []byte, f func(row *Row) bool) // todo: return error?
}
