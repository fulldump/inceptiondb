package api

import (
	"compress/gzip"
	"context"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/fulldump/box"
)

func Compression(next box.H) box.H {
	return func(ctx context.Context) {
		r := box.GetRequest(ctx)
		w := box.GetResponse(ctx)

		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next(ctx)
			return
		}
		mimeType := mime.TypeByExtension(filepath.Ext(r.URL.Path))
		if strings.HasPrefix(mimeType, "image/") {
			next(ctx)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		gzw := gzipResponseWriter{Writer: gz, ResponseWriter: w}
		box.GetBoxContext(ctx).Response = gzw
		next(ctx)
	}
}

// Gzip Compression
type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}
