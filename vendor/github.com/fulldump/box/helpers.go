package box

import (
	"context"
	"net/http"
)

func GetError(ctx context.Context) error {
	return GetBoxContext(ctx).error
}

func SetError(ctx context.Context, err error) {
	GetBoxContext(ctx).error = err
}

func GetBoxContext(ctx context.Context) *C {

	v := ctx.Value("box_context")
	if c, ok := v.(*C); ok {
		return c
	}

	return nil
}

func SetBoxContext(ctx context.Context, c *C) context.Context {
	return context.WithValue(ctx, "box_context", c)
}

// TODO: add missing helpers...

func GetResponse(ctx context.Context) http.ResponseWriter {
	return GetBoxContext(ctx).Response
}

func GetRequest(ctx context.Context) *http.Request {
	return GetBoxContext(ctx).Request
}

func GetUrlParameter(ctx context.Context, param string) string {
	return GetBoxContext(ctx).Parameters[param]
}

func Param(r *http.Request, param string) string {
	return GetUrlParameter(r.Context(), param)
}
