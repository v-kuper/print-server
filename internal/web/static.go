package web

import (
	_ "embed"
	"net/http"
)

//go:embed static/index.html
var indexHTML []byte

//go:embed static/app.css
var appCSS []byte

//go:embed static/app.js
var appJS []byte

func handleAppCSS(w http.ResponseWriter, r *http.Request) {
	writeStaticClientFile(w, "text/css; charset=utf-8", appCSS)
}

func handleAppJS(w http.ResponseWriter, r *http.Request) {
	writeStaticClientFile(w, "text/javascript; charset=utf-8", appJS)
}

func writeStaticClientFile(w http.ResponseWriter, contentType string, content []byte) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", contentType)
	_, _ = w.Write(content)
}

func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
