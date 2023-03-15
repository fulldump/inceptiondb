package box

import (
	"context"
	"runtime/debug"
)

// RecoverFromPanic is an interceptor to recover and pretty print a stacktrace
func RecoverFromPanic(next H) H {
	return func(ctx context.Context) {
		defer func() {
			if err := recover(); err != nil {
				debug.PrintStack()
			}
		}()
		next(ctx)
	}
}
