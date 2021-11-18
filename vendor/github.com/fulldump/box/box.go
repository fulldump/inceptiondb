package box

import (
	"fmt"
	"net/http"
)

type B struct {
	// R is the root resource in box
	*R
}

func NewBox() *B {
	return &B{
		R: NewResource(),
	}
}

func (b *B) Serve() {

	server := &http.Server{
		Addr:    ":8080",
		Handler: Box2Http(b),
	}

	fmt.Println("Listening to ", server.Addr)

	server.ListenAndServe()
}
