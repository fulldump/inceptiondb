//go:build !go1.22

package box

import (
	"net/http"
)

func fillPathValues(params map[string]string, r *http.Request) {
	// Do nothing
}
