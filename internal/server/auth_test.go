package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOAuthStartUsesStatePKCEAndSignedCookie(t *testing.T) {
	s := New(Config{GitHubID: "client", GitHubSecret: "secret", SessionSecret: strings.Repeat("s", 32)})
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/auth/github", nil))
	if r.Code != http.StatusFound || !strings.Contains(r.Header().Get("Location"), "code_challenge_method=S256") || !strings.Contains(r.Header().Get("Set-Cookie"), "HttpOnly") {
		t.Fatalf("status=%d location=%q cookie=%q", r.Code, r.Header().Get("Location"), r.Header().Get("Set-Cookie"))
	}
}

func TestMeRejectsTamperedSession(t *testing.T) {
	s := New(Config{SessionSecret: strings.Repeat("s", 32)})
	r := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	req.AddCookie(&http.Cookie{Name: "prompter_session", Value: "changed.invalid"})
	s.ServeHTTP(r, req)
	if r.Code != http.StatusOK || !strings.Contains(r.Body.String(), `"authenticated":false`) {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestOAuthProductionConfigRequiresHTTPSAndSessionSecret(t *testing.T) {
	values := map[string]string{"PROMPTPATCH_SERVER_ADDR": "0.0.0.0:8787", "PROMPTPATCH_SERVER_TOKEN": "token", "GITHUB_CLIENT_ID": "id", "GITHUB_CLIENT_SECRET": "secret"}
	if _, err := FromEnv(func(key string) string { return values[key] }); err == nil {
		t.Fatal("expected unsafe OAuth config to fail")
	}
}

func TestConfiguredOAuthProtectsWorkspace(t *testing.T) {
	s := New(Config{GitHubID: "id", GitHubSecret: "secret"})
	r := httptest.NewRecorder()
	s.ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/v1/workspace", nil))
	if r.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", r.Code, r.Body.String())
	}
}

func TestLocalSessionSurvivesServerRestart(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	first := New(Config{})
	user := authUser{ID: "1", Provider: "github", Name: "Dev", Expires: time.Now().Add(time.Hour).Unix()}
	data, _ := json.Marshal(user)
	response := httptest.NewRecorder()
	first.setSignedCookie(response, "prompter_session", string(data), time.Hour)
	request := httptest.NewRequest(http.MethodGet, "/v1/me", nil)
	request.AddCookie(response.Result().Cookies()[0])
	got := httptest.NewRecorder()
	New(Config{}).ServeHTTP(got, request)
	if !strings.Contains(got.Body.String(), `"authenticated":true`) {
		t.Fatalf("body=%s", got.Body.String())
	}
}

func TestMutationRejectsForeignOrigin(t *testing.T) {
	s := New(Config{})
	request := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	request.Header.Set("Origin", "https://evil.example")
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status=%d", response.Code)
	}
}
