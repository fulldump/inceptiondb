package api

import (
	"context"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
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

func AccessLog(l *log.Logger) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {
			r := box.GetRequest(ctx)
			now := time.Now()
			defer func() {
				l.Println(now.UTC().Format(time.RFC3339Nano), formatRemoteAddr(r), r.Method, r.URL.String(), time.Since(now))
			}()

			next(ctx)
		}
	}
}

func formatRemoteAddr(r *http.Request) string {
	xorigin := strings.TrimSpace(strings.Split(
		r.Header.Get("X-Forwarded-For"), ",")[0])
	if xorigin != "" {
		return xorigin
	}

	return r.RemoteAddr[0:strings.LastIndex(r.RemoteAddr, ":")]
}
