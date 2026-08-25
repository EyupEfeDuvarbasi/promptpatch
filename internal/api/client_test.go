package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestImproveCallsServerWithBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/improve" {
			t.Fatalf("path=%s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("authorization=%q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(`{"original_score":40,"improved_score":80,"improved_prompt":"src/parser.go dosyasını düzelt.","source":"ollama"}`))
	}))
	defer server.Close()
	client := Client{URL: server.URL, Token: "secret"}

	response, err := client.Improve(context.Background(), ImproveRequest{Prompt: "şunu düzelt"})

	if err != nil {
		t.Fatal(err)
	}
	if response.Source != "ollama" || !strings.Contains(response.ImprovedPrompt, "src/parser.go") {
		t.Fatalf("response=%#v", response)
	}
}

func TestImproveReturnsServerErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"yetkisiz"}`))
	}))
	defer server.Close()
	client := Client{URL: server.URL}

	_, err := client.Improve(context.Background(), ImproveRequest{Prompt: "şunu düzelt"})

	if err == nil || !strings.Contains(err.Error(), "yetkisiz") {
		t.Fatalf("err=%v", err)
	}
}

func TestImproveAcceptsFailedQualityStatusWithoutRewrite(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"original_score":40,"improved_prompt":"","source":"ollama","quality_status":"failed","quality_message":"çıktı tamamlanmadı"}`))
	}))
	defer server.Close()
	response, err := (Client{URL: server.URL}).Improve(context.Background(), ImproveRequest{Prompt: "şunu düzelt"})
	if err != nil || response.QualityStatus != "failed" || response.QualityMessage == "" {
		t.Fatalf("response=%#v err=%v", response, err)
	}
}
