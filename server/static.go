package server

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed static/*
var staticFS embed.FS

// spaFS holds the built Vite app. Populated when web/dist is copied to server/spa.
//
//go:embed all:spa
var spaFS embed.FS

func staticHandler() http.Handler {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(err)
	}
	return http.FileServer(http.FS(sub))
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	b, err := staticFS.ReadFile("static/index.html")
	if err != nil {
		http.Error(w, "index missing", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(b)
}

func spaSubFS() (fs.FS, error) {
	return fs.Sub(spaFS, "spa")
}

func tryReadSPA() ([]byte, error) {
	// Prefer index.html from embedded spa/
	if b, err := spaFS.ReadFile("spa/index.html"); err == nil && len(b) > 0 && !isPlaceholderSPA(b) {
		return b, nil
	}
	return nil, fs.ErrNotExist
}

func tryServeSPAFile(w http.ResponseWriter, r *http.Request) bool {
	sub, err := spaSubFS()
	if err != nil {
		return false
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" || path == "index.html" {
		return false
	}
	// only serve real files
	f, err := sub.Open(path)
	if err != nil {
		return false
	}
	_ = f.Close()
	http.FileServer(http.FS(sub)).ServeHTTP(w, r)
	return true
}

func isPlaceholderSPA(b []byte) bool {
	return strings.Contains(string(b), "SPA_PLACEHOLDER")
}
