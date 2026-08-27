package server

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFiles embed.FS

func (s *Server) webRoutes() {
	assets, _ := fs.Sub(webFiles, "web")
	files := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		http.FileServer(http.FS(assets)).ServeHTTP(w, r)
	})
	s.mux.Handle("GET /assets/", http.StripPrefix("/assets/", files))
	s.mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		data, _ := webFiles.ReadFile("web/index.html")
		_, _ = w.Write(data)
	})
}
