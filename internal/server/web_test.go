package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWebUI(t *testing.T) {
	s := New(Config{})
	for _, path := range []string{"/", "/assets/styles.css", "/assets/app.js"} {
		r := httptest.NewRecorder()
		s.ServeHTTP(r, httptest.NewRequest(http.MethodGet, path, nil))
		if r.Code != http.StatusOK || !strings.Contains(r.Header().Get("Content-Type"), map[string]string{"/": "text/html", "/assets/styles.css": "text/css", "/assets/app.js": "javascript"}[path]) {
			t.Fatalf("%s status=%d type=%q", path, r.Code, r.Header().Get("Content-Type"))
		}
	}
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/assets/app.js", nil))
	if r.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("asset cache=%q", r.Header().Get("Cache-Control"))
	}
	app, _ := webFiles.ReadFile("web/app.js")
	for _, required := range []string{"Google ile devam et", "Continue with Google", "GitHub ile devam et", "Continue with GitHub", "function translateDOM"} {
		if !strings.Contains(string(app), required) {
			t.Fatalf("web UI missing %q", required)
		}
	}
}
