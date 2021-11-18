package box

import (
	"context"
	"net/http"
)

func GetError(ctx context.Context) error {
	return getBoxContext(ctx).error
}

func SetError(ctx context.Context, err error) {
	getBoxContext(ctx).error = err
}

func getBoxContext(ctx context.Context) *C {

	v := ctx.Value("box_context")
	if c, ok := v.(*C); ok {
		return c
	}

	return nil
}

func setBoxContext(ctx context.Context, c *C) context.Context {
	return context.WithValue(ctx, "box_context", c)
}

// TODO: add missing helpers...

func GetResponse(ctx context.Context) http.ResponseWriter {
	return getBoxContext(ctx).Response
}

func GetRequest(ctx context.Context) *http.Request {
	return getBoxContext(ctx).Request
}
