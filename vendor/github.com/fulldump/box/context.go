package box

import "net/http"

// An C is a box context to store box related thing in context such as
// *R, *A, *E, etc
type C struct {
	error
	Resource   *R
	Action     *A
	Parameters map[string]string
	// TODO: add headers
	// TODO: add query
	// TODO: add box
	// TODO: ¿add marshaler and unmarshaler?
	Request  *http.Request
	Response http.ResponseWriter
}
