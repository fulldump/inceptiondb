package api

import (
	"context"
	"log"
	"runtime/debug"
	"time"

	"github.com/fulldump/box"
)

func recoverFromPanic(next box.H) box.H {
	return func(ctx context.Context) {
		go func() {
			if err := recover(); err != nil {
				debug.PrintStack()
			}
		}()
		next(ctx)
	}
}

func accessLog(l *log.Logger) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {
			r := box.GetRequest(ctx)
			now := time.Now()
			defer l.Println(r.Method, r.URL.String(), time.Since(now))
			next(ctx)
		}
	}
}
