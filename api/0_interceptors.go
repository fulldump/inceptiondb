package api

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/fulldump/box"
)

var ErrUnauthorized = errors.New("unauthorized")

func Authenticate(apiKey, apiSecret string) box.I {
	return func(next box.H) box.H {
		return func(ctx context.Context) {

			if apiKey == "" && apiSecret == "" {
				next(ctx)
				return
			}

			r := box.GetRequest(ctx)
			key := r.Header.Get("X-Api-Key")
			secret := r.Header.Get("X-Api-Secret")

			if key != apiKey || secret != apiSecret {
				box.SetError(ctx, ErrUnauthorized)
				return
			}
			next(ctx)
		}
	}
}

func RecoverFromPanic(next box.H) box.H {
	return func(ctx context.Context) {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("ERROR:", err)
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
			action := ""
			if boxAction := box.GetBoxContext(ctx).Action; boxAction != nil {
				action = boxAction.Name
			}
			now := time.Now()
			defer func() {
				l.Println(now.UTC().Format(time.RFC3339Nano), formatRemoteAddr(r), r.Method, r.URL.String(), time.Since(now), action)
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
