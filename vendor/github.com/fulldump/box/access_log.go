package box

import (
	"context"
	"log"
)

var DefaultAccessLogPrintln = log.Println

func AccessLog(next H) H {
	return func(ctx context.Context) {
		r := GetRequest(ctx)
		DefaultAccessLogPrintln(r.Method, r.URL.String())
		next(ctx)
	}
}
