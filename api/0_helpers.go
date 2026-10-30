package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/fulldump/box"

	"github.com/fulldump/inceptiondb/database"
	"github.com/fulldump/inceptiondb/service"
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
		w.Header().Set("Content-Type", "application/json")

		if errors.Is(err, service.ErrorCollectionNotFound) {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]interface{}{
					"message":     err.Error(),
					"description": "The collection does not exist.",
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

		if isMalformedJSON(err) {
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

func isMalformedJSON(err error) bool {
	var syntaxError *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	return errors.As(err, &syntaxError) ||
		errors.As(err, &typeError) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}
