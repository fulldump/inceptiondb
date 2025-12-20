package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/database"
)

func getBoxContext(ctx context.Context) *box.C {

	v := ctx.Value("box_context")
	if c, ok := v.(*box.C); ok {
		return c
	}

	return nil
}

func interceptorPrintError(next box.H) box.H {
	return func(ctx context.Context) {
		next(ctx)
		err := box.GetError(ctx)
		if nil != err {
			json.NewEncoder(box.GetResponse(ctx)).Encode(map[string]interface{}{
				"error": err.Error(),
			})
		}
	}
}

type PrettyError struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

func (p PrettyError) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"error": struct {
			Message     string `json:"message"`
			Description string `json:"description"`
		}{
			p.Message,
			p.Description,
		},
	})
}

func (p PrettyError) MarshalTo(w io.Writer) error {
	return json.NewEncoder(w).Encode(p)
}

func InterceptorUnavailable(db *database.Database) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {

			status := db.GetStatus()
			if status == database.StatusOpening {
				box.SetError(ctx, fmt.Errorf("temporary unavailable: opening"))
				return
			}
			if status == database.StatusClosing {
				box.SetError(ctx, fmt.Errorf("temporary unavailable: closing"))
				return
			}
			next(ctx)
		}
	}
}

func PrettyErrorInterceptor(next box.H) box.H {
	return func(ctx context.Context) {

		next(ctx)

		err := box.GetError(ctx)
		if err == nil {
			return
		}
		w := box.GetResponse(ctx)

		if err == ErrUnauthorized {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("user is not authenticated"),
				},
			})
			return
		}

		if err == box.ErrResourceNotFound {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("resource '%s' not found", box.GetRequest(ctx).URL.String()),
				},
			})
			return
		}

		if err == box.ErrMethodNotAllowed {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": fmt.Sprintf("method '%s' not allowed", box.GetRequest(ctx).Method),
				},
			})
			return
		}

		if _, ok := err.(*json.SyntaxError); ok {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": "Malformed JSON",
				},
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"message":     err.Error(),
				"description": "Unexpected error",
			},
		})

	}
}
