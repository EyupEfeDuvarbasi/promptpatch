package server

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPasswordRegistrationAndLogin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{SessionSecret: strings.Repeat("s", 32)})
	register := httptest.NewRecorder()
	s.ServeHTTP(register, httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{"email":"dev@example.com","password":"correct horse battery staple","name":"Dev"}`)))
	if register.Code != http.StatusCreated || len(register.Result().Cookies()) == 0 {
		t.Fatalf("register status=%d body=%s", register.Code, register.Body.String())
	}
	path := filepath.Join(os.Getenv("HOME"), ".local", "share", "prompter", "users.json")
	stored, err := os.ReadFile(path)
	if err != nil || strings.Contains(string(stored), "correct horse battery staple") {
		t.Fatalf("unsafe password store: %v %s", err, stored)
	}
	login := httptest.NewRecorder()
	s.ServeHTTP(login, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"dev@example.com","password":"correct horse battery staple","name":""}`)))
	if login.Code != http.StatusOK || len(login.Result().Cookies()) == 0 {
		t.Fatalf("login status=%d body=%s", login.Code, login.Body.String())
	}
}
