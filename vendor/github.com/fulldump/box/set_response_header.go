package box

import (
	"context"
)

func SetResponseHeader(key, value string) I {
	return func(next H) H {
		return func(ctx context.Context) {
			GetResponse(ctx).Header().Set(key, value)
			next(ctx)
		}
	}
}
