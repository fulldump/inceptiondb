package statics

import (
	"embed"
	"net/http"
	"net/url"
)

// Serve static files
//go:embed www/*
var www embed.FS

func ServeStatics(staticsDir string) http.HandlerFunc {
	if staticsDir == "" {
		return AddPrefix("../www", http.FileServer(http.FS(www)))
	}
	return http.FileServer(http.Dir(staticsDir)).ServeHTTP
}

// Copied from http.StripPrefix
func AddPrefix(prefix string, h http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := prefix + r.URL.Path
		rp := prefix + r.URL.Path
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = new(url.URL)
		*r2.URL = *r.URL
		r2.URL.Path = p
		r2.URL.RawPath = rp
		h.ServeHTTP(w, r2)
	}
}
